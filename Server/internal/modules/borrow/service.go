package borrow

import (
	"context"
	"time"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/modules/catalog"
	"github.com/dprince-03/Bibliotheca/internal/utils"
)

type Service struct {
	borrowRepo Repository
	bookRepo   catalog.BookRepository
	loanDays   int
}

func NewService(borrowRepo Repository, bookRepo catalog.BookRepository, loanDays int) *Service {
	return &Service{
		borrowRepo: borrowRepo,
		bookRepo:   bookRepo,
		loanDays:   loanDays,
	}
}

// ── List ──────────────────────────────────────────────────

// GetAll lists every borrow record (admin/librarian). Sweeps overdue
// records first, per the roadmap's "auto-detect overdue on fetch" — a
// read-time sweep instead of a scheduled job, since there's no cron/worker
// in this codebase yet.
func (s *Service) GetAll(ctx context.Context, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	if err := s.borrowRepo.UpdateOverdue(ctx); err != nil {
		return nil, err
	}

	records, total, err := s.borrowRepo.GetAll(ctx, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	items, err := s.enrichBorrows(ctx, records)
	if err != nil {
		return nil, err
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)
	return &resp, nil
}

// GetMyBorrows lists the caller's own borrow records.
func (s *Service) GetMyBorrows(ctx context.Context, userID uint64, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	if err := s.borrowRepo.UpdateOverdue(ctx); err != nil {
		return nil, err
	}

	records, total, err := s.borrowRepo.GetAllByUserID(ctx, userID, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	items, err := s.enrichBorrows(ctx, records)
	if err != nil {
		return nil, err
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)
	return &resp, nil
}

// ── Borrow ────────────────────────────────────────────────

// Borrow creates a new borrow record for the caller. DecrementAvailableCopies
// is the actual atomic "reserve a copy" check-and-update (WHERE available_copies
// > 0) — the HasActiveBorrow check beforehand is a business-rule guard (don't
// let someone borrow the same book twice), not the concurrency safeguard.
// If creating the record fails after the copy was reserved, the decrement is
// compensated (incremented back) so a failed borrow never leaks a copy.
func (s *Service) Borrow(ctx context.Context, userID uint64, req BorrowRequest) (*BorrowResponse, error) {
	book, err := s.bookRepo.GetByID(ctx, req.BookID)
	if err != nil {
		return nil, err
	}

	hasActive, err := s.borrowRepo.HasActiveBorrow(ctx, userID, req.BookID)
	if err != nil {
		return nil, err
	}
	if hasActive {
		return nil, apperrors.Conflict("you already have an active borrow for this book")
	}

	if err := s.bookRepo.DecrementAvailableCopies(ctx, req.BookID); err != nil {
		return nil, err
	}

	record := &BorrowRecord{
		UserID: userID,
		BookID: req.BookID,
		DueAt:  time.Now().AddDate(0, 0, s.loanDays),
	}

	id, err := s.borrowRepo.Create(ctx, record)
	if err != nil {
		_ = s.bookRepo.IncrementAvailableCopies(ctx, req.BookID)
		return nil, err
	}
	record.ID = id
	record.Status = "active"
	record.BorrowedAt = time.Now()

	resp := mapBorrowToResponse(record, book.Title)
	return &resp, nil
}

// ── Return ────────────────────────────────────────────────

// Return marks a borrow as returned. A member may only return their own
// borrow; librarian/admin may process a return on anyone's behalf (e.g. at
// a physical returns desk).
func (s *Service) Return(ctx context.Context, userID uint64, role string, borrowID uint64) error {
	record, err := s.borrowRepo.GetByID(ctx, borrowID)
	if err != nil {
		return err
	}

	isSelf := record.UserID == userID
	isStaff := role == "librarian" || role == "admin"
	if !isSelf && !isStaff {
		return apperrors.Forbidden("you do not have permission to return this borrow")
	}

	if record.Status == "returned" {
		return apperrors.Conflict("this borrow has already been returned")
	}

	if err := s.borrowRepo.MarkReturned(ctx, borrowID); err != nil {
		return err
	}

	return s.bookRepo.IncrementAvailableCopies(ctx, record.BookID)
}

// ── Helpers ───────────────────────────────────────────────

func (s *Service) enrichBorrows(ctx context.Context, records []*BorrowRecord) ([]BorrowResponse, error) {
	items := make([]BorrowResponse, len(records))
	for i, r := range records {
		title := ""
		if book, err := s.bookRepo.GetByID(ctx, r.BookID); err == nil {
			title = book.Title
		}
		items[i] = mapBorrowToResponse(r, title)
	}
	return items, nil
}

func mapBorrowToResponse(r *BorrowRecord, bookTitle string) BorrowResponse {
	return BorrowResponse{
		ID:         r.ID,
		BookID:     r.BookID,
		BookTitle:  bookTitle,
		BorrowedAt: r.BorrowedAt,
		DueAt:      r.DueAt,
		ReturnedAt: r.ReturnedAt,
		Status:     r.Status,
	}
}
