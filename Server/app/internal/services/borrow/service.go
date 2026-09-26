package borrow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/google/uuid"
)

const serviceName = "borrow-service"

// Catalog is the slice of catalog-service borrow-service depends on — its
// only cross-service dependency. An interface so the Saga can be tested
// without a network (see service_test.go).
type Catalog interface {
	// BookTitle validates the book exists and returns its current title
	// (snapshotted onto the borrow record).
	BookTitle(ctx context.Context, bookID uint64) (string, error)
	// ReserveCopy returns grpcx.ErrNoCopies-wrapping errors when nothing is
	// available. Idempotent per key.
	ReserveCopy(ctx context.Context, bookID uint64, key string) error
	// ReleaseCopy is idempotent per key.
	ReleaseCopy(ctx context.Context, bookID uint64, key string) error
}

type Service struct {
	borrowRepo Repository
	catalog    Catalog
	loanDays   int
	now        func() time.Time
}

func NewService(borrowRepo Repository, catalog Catalog, loanDays int) *Service {
	return &Service{
		borrowRepo: borrowRepo,
		catalog:    catalog,
		loanDays:   loanDays,
		now:        time.Now,
	}
}

// ── List ──────────────────────────────────────────────────

// GetAll lists every borrow record (admin/librarian). Sweeps overdue
// records first so status is current as of this request; the background
// sweeper (RunOverdueSweeper) does the same on a timer, so overdue emails
// go out even when nobody is looking at a list.
func (s *Service) GetAll(ctx context.Context, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	if _, err := s.borrowRepo.MarkOverdue(ctx, overdueEvent(ctx)); err != nil {
		return nil, err
	}

	records, total, err := s.borrowRepo.GetAll(ctx, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	resp := utils.NewPaginatedResponse(mapBorrows(records), total, pg.Page, pg.Limit)
	return &resp, nil
}

// GetMyBorrows lists the caller's own borrow records.
func (s *Service) GetMyBorrows(ctx context.Context, userID uint64, pg utils.Pagination) ([]BorrowResponse, int, error) {
	if _, err := s.borrowRepo.MarkOverdue(ctx, overdueEvent(ctx)); err != nil {
		return nil, 0, err
	}

	records, total, err := s.borrowRepo.GetAllByUserID(ctx, userID, pg.Limit, pg.Offset)
	if err != nil {
		return nil, 0, err
	}
	return mapBorrows(records), total, nil
}

// GetMyOpenBorrow returns the caller's active/overdue borrow of one book.
func (s *Service) GetMyOpenBorrow(ctx context.Context, userID, bookID uint64) (*BorrowResponse, error) {
	record, err := s.borrowRepo.GetOpenByUserAndBook(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	resp := mapBorrowToResponse(record)
	return &resp, nil
}

// ── Borrow (the Saga) ─────────────────────────────────────

// Borrow runs the orchestrated Saga:
//
//  1. idempotency replay — same (user, key) already borrowed? return it;
//  2. business guard — no second open borrow of the same book;
//  3. catalog.BookTitle — the book exists (and its title, to snapshot);
//  4. catalog.ReserveCopy — atomically take a copy (the real concurrency
//     guard, keyed so a retry can't take two);
//  5. create the record + borrow.book_borrowed outbox event (one tx);
//  6. on failure after 4, compensate with catalog.ReleaseCopy — inline if
//     catalog is reachable, else queued for RunCompensator to retry.
//
// created is false when step 1 replayed an earlier result.
func (s *Service) Borrow(ctx context.Context, userID uint64, email string, req BorrowRequest, idempotencyKey string) (resp *BorrowResponse, created bool, err error) {
	if idempotencyKey != "" {
		if existing, err := s.borrowRepo.GetByIdempotencyKey(ctx, userID, idempotencyKey); err == nil {
			r := mapBorrowToResponse(existing)
			return &r, false, nil
		}
	} else {
		idempotencyKey = uuid.NewString()
	}

	hasOpen, err := s.borrowRepo.HasOpenBorrow(ctx, userID, req.BookID)
	if err != nil {
		return nil, false, err
	}
	if hasOpen {
		return nil, false, ErrOpenBorrowExists
	}

	title, err := s.catalog.BookTitle(ctx, req.BookID)
	if err != nil {
		return nil, false, err
	}

	reservationKey := fmt.Sprintf("borrow:%d:%s", userID, idempotencyKey)
	if err := s.catalog.ReserveCopy(ctx, req.BookID, reservationKey); err != nil {
		if isUnavailable(err) {
			// Unknown outcome: catalog may have reserved before the
			// connection dropped. Releasing is safe either way (idempotent,
			// no-op if nothing was reserved under this key).
			s.compensate(ctx, req.BookID, reservationKey, err)
		}
		return nil, false, err
	}

	record := &BorrowRecord{
		PublicID:       utils.NewPublicID(),
		UserID:         userID,
		UserEmail:      email,
		BookID:         req.BookID,
		BookTitle:      title,
		ReservationKey: reservationKey,
		IdempotencyKey: idempotencyKey,
		DueAt:          dueAt(s.now(), s.loanDays),
	}
	if _, err := s.borrowRepo.Create(ctx, record, borrowEvent(ctx, events.TypeBookBorrowed)); err != nil {
		s.compensate(ctx, req.BookID, reservationKey, err)
		return nil, false, err
	}

	r := mapBorrowToResponse(record)
	return &r, true, nil
}

// compensate releases a reserved copy after a failed borrow. It runs on a
// context detached from the request's cancellation (a client hanging up
// must not stop the cleanup); if catalog can't be reached, the release is
// queued in saga_compensations for RunCompensator.
func (s *Service) compensate(ctx context.Context, bookID uint64, reservationKey string, cause error) {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if err := s.catalog.ReleaseCopy(cctx, bookID, reservationKey); err == nil {
		slog.InfoContext(ctx, "saga compensated: copy released", "book_id", bookID, "reservation_key", reservationKey, "cause", cause)
		return
	} else if qerr := s.borrowRepo.AddCompensation(cctx, reservationKey, bookID, err); qerr != nil {
		slog.ErrorContext(ctx, "saga compensation failed AND could not be queued — copy leaked",
			"book_id", bookID, "reservation_key", reservationKey, "release_error", err, "queue_error", qerr)
	} else {
		slog.WarnContext(ctx, "saga compensation queued for retry", "book_id", bookID, "reservation_key", reservationKey, "error", err)
	}
}

// RunCompensator retries queued compensations until ctx ends.
func (s *Service) RunCompensator(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		due, err := s.borrowRepo.DueCompensations(ctx, 20)
		if err != nil {
			slog.Warn("loading saga compensations failed", "error", err)
			continue
		}
		for _, c := range due {
			if err := s.catalog.ReleaseCopy(ctx, c.BookID, c.ReservationKey); err != nil {
				_ = s.borrowRepo.CompensationFailed(ctx, c.ReservationKey, err)
				continue
			}
			_ = s.borrowRepo.CompensationDone(ctx, c.ReservationKey)
			slog.Info("queued saga compensation succeeded", "reservation_key", c.ReservationKey, "attempts", c.Attempts+1)
		}
	}
}

// RunOverdueSweeper flips overdue borrows on a timer (not only on reads),
// so borrow.borrow_overdue — and the reminder email — fires on time.
func (s *Service) RunOverdueSweeper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := s.borrowRepo.MarkOverdue(ctx, overdueEvent(ctx)); err != nil {
				slog.Warn("overdue sweep failed", "error", err)
			} else if n > 0 {
				slog.Info("overdue sweep", "flipped", n)
			}
		}
	}
}

// ── Return ────────────────────────────────────────────────

// Return marks a borrow as returned. A member may only return their own
// borrow; librarian/admin may process a return on anyone's behalf (e.g. at
// a physical returns desk).
//
// The copy is released on catalog-service FIRST, then the record is
// marked returned. If the second step fails the member simply retries:
// ReleaseCopy is idempotent per reservation key, so the copy is never
// released twice — and it can never be stuck "returned but not released".
func (s *Service) Return(ctx context.Context, userID uint64, role string, borrowID uint64) (*BorrowResponse, error) {
	record, err := s.borrowRepo.GetByID(ctx, borrowID)
	if err != nil {
		return nil, err
	}
	return s.returnRecord(ctx, userID, role, record)
}

// ReturnByPublicID is Return addressed by the record's public UUID.
func (s *Service) ReturnByPublicID(ctx context.Context, userID uint64, role string, publicID string) (*BorrowResponse, error) {
	record, err := s.borrowRepo.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	return s.returnRecord(ctx, userID, role, record)
}

func (s *Service) returnRecord(ctx context.Context, userID uint64, role string, record *BorrowRecord) (*BorrowResponse, error) {

	isSelf := record.UserID == userID
	isStaff := role == middleware.RoleLibrarian || role == middleware.RoleAdmin
	if !isSelf && !isStaff {
		return nil, apperrors.Forbidden("you do not have permission to return this borrow")
	}

	if record.Status == StatusReturned {
		return nil, apperrors.Conflict("this borrow has already been returned")
	}

	if err := s.catalog.ReleaseCopy(ctx, record.BookID, record.ReservationKey); err != nil {
		return nil, err
	}

	if err := s.borrowRepo.MarkReturned(ctx, record, borrowEvent(ctx, events.TypeBookReturned)); err != nil {
		return nil, err
	}

	resp := mapBorrowToResponse(record)
	return &resp, nil
}

// ── Helpers ───────────────────────────────────────────────

func borrowEvent(ctx context.Context, eventType string) EventFor {
	return func(r *BorrowRecord) (events.Event, error) {
		return events.New(ctx, eventType, serviceName, utils.Uint64Key(r.ID), events.BorrowEvent{
			BorrowID:  r.ID,
			UserID:    r.UserID,
			UserEmail: r.UserEmail,
			BookID:    r.BookID,
			BookTitle: r.BookTitle,
			DueAt:     r.DueAt,
		})
	}
}

func overdueEvent(ctx context.Context) EventFor {
	return borrowEvent(ctx, events.TypeBorrowOverdue)
}

func mapBorrows(records []*BorrowRecord) []BorrowResponse {
	items := make([]BorrowResponse, len(records))
	for i, r := range records {
		items[i] = mapBorrowToResponse(r)
	}
	return items
}

// mapBorrowToResponse uses the title snapshotted at borrow time — no
// per-record catalog lookup (the pre-split enrichBorrows did one per row).
func mapBorrowToResponse(r *BorrowRecord) BorrowResponse {
	return BorrowResponse{
		ID:         r.ID,
		PublicID:   r.PublicID,
		BookID:     r.BookID,
		BookTitle:  r.BookTitle,
		BorrowedAt: r.BorrowedAt,
		DueAt:      r.DueAt,
		ReturnedAt: r.ReturnedAt,
		Status:     r.Status,
	}
}
