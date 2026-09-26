package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Reconciliation: user-service keeps a copy of every account auth-service
// owns, fed by events (outbox + deduped consumers, so it shouldn't drift).
// This job is the safety net for when it does anyway — a manual DB edit, a
// bug, a restore from a different point in time — comparing both sides and
// repairing by each field's single owner:
//
//   - account missing here        → create it from auth's record
//   - email / name / role differ  → take auth's values (auth writes those)
//   - is_active differs           → user-service writes is_active, so
//     re-publish user.status_changed and let auth converge
//   - user here that auth doesn't have → can't be repaired safely; flagged
//     (log + metric) for a human
//
// One replica runs it at a time (a Postgres advisory lock).

var (
	reconcileRepairs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "reconcile_repairs_total",
		Help: "Account/user drift found by the reconciler, by kind.",
	}, []string{"kind"})
	reconcileRuns = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "reconcile_runs_total",
		Help: "Reconciler runs by outcome.",
	}, []string{"outcome"})
)

const reconcileLockKey = 7_211_001 // arbitrary, unique to this job

// AccountSource lists auth-service's accounts, paged by id.
type AccountSource interface {
	ListAccounts(ctx context.Context, afterID uint64, limit int) ([]Account, error)
}

// Account is auth's view of one account (mirrors auth.AccountRecord).
type Account struct {
	ID        uint64 `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
}

// ReconcileReport summarizes one run.
type ReconcileReport struct {
	Checked, Created, Updated, StatusRepublished, Orphans int
}

type Reconciler struct {
	db     *sqlx.DB
	source AccountSource
}

func NewReconciler(db *sqlx.DB, source AccountSource) *Reconciler {
	return &Reconciler{db: db, source: source}
}

// reconcileStartDelay: the first pass runs shortly after boot rather than a
// full interval later, so drift is repaired on every deploy — but not
// instantly, giving auth-service time to be reachable during a rollout.
const reconcileStartDelay = 30 * time.Second

// Run reconciles once shortly after start, then every interval until ctx ends.
func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	slog.InfoContext(ctx, "account reconciler started", "interval", interval.String(), "first_run_in", reconcileStartDelay.String())
	timer := time.NewTimer(reconcileStartDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.runOnce(ctx)
			timer.Reset(interval)
		}
	}
}

func (r *Reconciler) runOnce(ctx context.Context) {
	rep, ran, err := r.ReconcileOnce(ctx)
	switch {
	case err != nil:
		reconcileRuns.WithLabelValues("error").Inc()
		slog.WarnContext(ctx, "reconciliation failed", "error", err)
	case ran:
		reconcileRuns.WithLabelValues("ok").Inc()
		slog.InfoContext(ctx, "reconciliation complete", "checked", rep.Checked, "created", rep.Created,
			"updated", rep.Updated, "status_republished", rep.StatusRepublished, "orphans", rep.Orphans)
	default:
		slog.DebugContext(ctx, "reconciliation skipped: another replica holds the lock")
	}
}

// ReconcileOnce runs one pass. ran is false if another replica holds the lock.
func (r *Reconciler) ReconcileOnce(ctx context.Context) (rep ReconcileReport, ran bool, err error) {
	conn, err := r.db.Connx(ctx) // advisory locks are per-connection
	if err != nil {
		return rep, false, err
	}
	defer conn.Close()
	var locked bool
	if err := conn.GetContext(ctx, &locked, `SELECT pg_try_advisory_lock($1)`, reconcileLockKey); err != nil {
		return rep, false, err
	}
	if !locked {
		return rep, false, nil
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, reconcileLockKey)
	}()

	seen := map[uint64]bool{}
	var after uint64
	for {
		page, err := r.source.ListAccounts(ctx, after, 500)
		if err != nil {
			return rep, true, fmt.Errorf("listing auth accounts: %w", err)
		}
		if len(page) == 0 {
			break
		}
		for _, a := range page {
			seen[a.ID] = true
			if err := r.reconcileAccount(ctx, a, &rep); err != nil {
				return rep, true, err
			}
			after = a.ID
		}
	}

	// Users auth has never heard of.
	var ids []uint64
	if err := r.db.SelectContext(ctx, &ids, `SELECT id FROM users ORDER BY id`); err != nil {
		return rep, true, err
	}
	for _, id := range ids {
		if !seen[id] {
			rep.Orphans++
			reconcileRepairs.WithLabelValues("orphan").Inc()
			slog.ErrorContext(ctx, "user exists without an auth account — needs manual review", "user_id", id)
		}
	}
	return rep, true, nil
}

func (r *Reconciler) reconcileAccount(ctx context.Context, a Account, rep *ReconcileReport) error {
	rep.Checked++
	var u User
	err := r.db.GetContext(ctx, &u, `SELECT `+userColumns+` FROM users WHERE id = $1`, a.ID)
	switch {
	case err != nil && isNoRows(err):
		return events.InTx(ctx, r.db, func(tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO users (id, first_name, last_name, email, role, is_active)
				VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`,
				a.ID, a.FirstName, a.LastName, a.Email, a.Role, a.IsActive); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO users_profile (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, a.ID); err != nil {
				return err
			}
			rep.Created++
			reconcileRepairs.WithLabelValues("missing_user").Inc()
			slog.WarnContext(ctx, "reconciler created missing user", "user_id", a.ID)
			return nil
		})
	case err != nil:
		return err
	}

	if u.Email != a.Email || u.FirstName != a.FirstName || u.LastName != a.LastName || u.Role != a.Role {
		if _, err := r.db.ExecContext(ctx, `
			UPDATE users SET email = $1, first_name = $2, last_name = $3, role = $4 WHERE id = $5`,
			a.Email, a.FirstName, a.LastName, a.Role, a.ID); err != nil {
			return err
		}
		rep.Updated++
		reconcileRepairs.WithLabelValues("identity_drift").Inc()
		slog.WarnContext(ctx, "reconciler corrected identity drift from auth", "user_id", a.ID)
	}

	if u.IsActive != a.IsActive {
		e, err := events.New(ctx, events.TypeUserStatusChanged, serviceName, utils.Uint64Key(a.ID),
			events.UserStatusChanged{UserID: a.ID, IsActive: u.IsActive})
		if err != nil {
			return err
		}
		if err := events.InTx(ctx, r.db, func(tx *sqlx.Tx) error { return events.AddToOutbox(ctx, tx, e) }); err != nil {
			return err
		}
		rep.StatusRepublished++
		reconcileRepairs.WithLabelValues("status_drift").Inc()
		slog.WarnContext(ctx, "reconciler re-published is_active to auth", "user_id", a.ID, "is_active", u.IsActive)
	}
	return nil
}

// ── auth-service client ────────────────────────────────────

type authAccounts struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewAuthAccountSource reads auth's internal accounts endpoint.
func NewAuthAccountSource(authURL, token string) AccountSource {
	return &authAccounts{baseURL: authURL, token: token, client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *authAccounts) ListAccounts(ctx context.Context, afterID uint64, limit int) ([]Account, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/internal/v1/accounts?after_id=%d&limit=%d", a.baseURL, afterID, limit), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", a.token)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth accounts endpoint: HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Data []Account `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
