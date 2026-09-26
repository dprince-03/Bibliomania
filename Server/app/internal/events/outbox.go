package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dprince-03/Bibliomania/internal/database"
	"github.com/dprince-03/Bibliomania/internal/telemetry"

	"github.com/jmoiron/sqlx"
)

// Transactional outbox: a service writes its business row AND the event
// describing it in the same local transaction, and a background relay
// publishes pending events afterwards. This closes the "row committed but
// broker was down, event lost" (and the reverse) gap that publishing
// directly after commit has — the outbox answers the split's "cross-service
// consistency strategy" question for every SQL-backed service.
//
// Each SQL service's migrations create the same two tables:
//
//	outbox_events    (id, event_id, event_type, payload, created_at, published_at)
//	processed_events (event_id, processed_at)   -- consumer-side dedupe

// AddToOutbox records e inside tx. Commit tx to make it visible to the relay.
func AddToOutbox(ctx context.Context, tx *sqlx.Tx, e Event) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}
	_, err = tx.ExecContext(ctx, tx.Rebind(
		`INSERT INTO outbox_events (event_id, event_type, payload) VALUES (?, ?, ?)`),
		e.ID, e.Type, string(payload))
	return err
}

// RunRelay polls the outbox and publishes pending events in order, until
// ctx is cancelled. Rows are locked with SKIP LOCKED, so running more than
// one replica of a service doesn't double-publish.
func RunRelay(ctx context.Context, db *sqlx.DB, pub Publisher, interval time.Duration) {
	slog.Info("outbox relay started")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanup.C:
			// Published rows are only kept for debugging; a week is plenty.
			if _, err := db.ExecContext(ctx, db.Rebind(
				`DELETE FROM outbox_events WHERE published_at IS NOT NULL AND published_at < ?`),
				time.Now().Add(-7*24*time.Hour)); err != nil {
				slog.Warn("outbox cleanup failed", "error", err)
			}
		case <-ticker.C:
			for {
				n, err := relayBatch(ctx, db, pub)
				if err != nil {
					slog.Warn("outbox relay batch failed", "error", err)
					break
				}
				if n < relayBatchSize {
					break
				}
			}
		}
	}
}

const relayBatchSize = 50

type outboxRow struct {
	ID      uint64 `db:"id"`
	Payload string `db:"payload"`
}

func relayBatch(ctx context.Context, db *sqlx.DB, pub Publisher) (int, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var rows []outboxRow
	if err := tx.SelectContext(ctx, &rows, tx.Rebind(fmt.Sprintf(
		`SELECT id, payload FROM outbox_events
		 WHERE published_at IS NULL
		 ORDER BY id
		 LIMIT %d
		 FOR UPDATE SKIP LOCKED`, relayBatchSize))); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	var published []uint64
	var pubErr error
	for _, row := range rows {
		e, err := parse([]byte(row.Payload))
		if err != nil {
			// A row we wrote ourselves can't be malformed unless the code
			// is broken — mark it published so it can't wedge the queue.
			slog.Error("dropping malformed outbox row", "id", row.ID, "error", err)
			published = append(published, row.ID)
			continue
		}
		if err := pub.Publish(ctx, e); err != nil {
			telemetry.EventsPublished.WithLabelValues(e.Type, "error").Inc()
			// Stop at the first failure to keep per-service event order.
			pubErr = fmt.Errorf("publishing %s %s: %w", e.Type, e.ID, err)
			break
		}
		telemetry.EventsPublished.WithLabelValues(e.Type, "ok").Inc()
		published = append(published, row.ID)
	}

	if len(published) > 0 {
		query, args, err := sqlx.In(`UPDATE outbox_events SET published_at = ? WHERE id IN (?)`, time.Now().UTC(), published)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, tx.Rebind(query), args...); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(published), pubErr
}

// MarkProcessed records that this consumer has handled eventID, inside the
// same transaction as the handler's own writes. It returns false if the
// event was already processed — the caller should then skip its work,
// which is what makes redelivered messages harmless.
func MarkProcessed(ctx context.Context, tx *sqlx.Tx, eventID string) (bool, error) {
	var query string
	switch tx.DriverName() {
	case database.DriverMySQL:
		query = `INSERT IGNORE INTO processed_events (event_id) VALUES (?)`
	default:
		query = `INSERT INTO processed_events (event_id) VALUES (?) ON CONFLICT DO NOTHING`
	}
	res, err := tx.ExecContext(ctx, tx.Rebind(query), eventID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// InTx runs fn in a transaction, committing on success.
func InTx(ctx context.Context, db *sqlx.DB, fn func(tx *sqlx.Tx) error) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("rollback failed", "error", rbErr)
		}
		return err
	}
	return tx.Commit()
}
