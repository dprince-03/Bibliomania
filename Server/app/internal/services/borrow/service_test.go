package borrow

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
)

// ── Fakes ─────────────────────────────────────────────────

type fakeCatalog struct {
	title      string
	reserveErr error
	releaseErr error
	reserved   map[string]bool // key -> currently held
	calls      []string
}

func newFakeCatalog() *fakeCatalog {
	return &fakeCatalog{title: "Dune", reserved: map[string]bool{}}
}

func (c *fakeCatalog) BookTitle(ctx context.Context, bookID uint64) (string, error) {
	c.calls = append(c.calls, "title")
	return c.title, nil
}

func (c *fakeCatalog) ReserveCopy(ctx context.Context, bookID uint64, key string) error {
	c.calls = append(c.calls, "reserve")
	if c.reserveErr != nil {
		return c.reserveErr
	}
	c.reserved[key] = true
	return nil
}

func (c *fakeCatalog) ReleaseCopy(ctx context.Context, bookID uint64, key string) error {
	c.calls = append(c.calls, "release")
	if c.releaseErr != nil {
		return c.releaseErr
	}
	delete(c.reserved, key)
	return nil
}

type fakeRepo struct {
	records       map[uint64]*BorrowRecord
	nextID        uint64
	createErr     error
	compensations map[string]uint64
	markErr       error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{records: map[uint64]*BorrowRecord{}, compensations: map[string]uint64{}}
}

func (r *fakeRepo) GetByID(ctx context.Context, id uint64) (*BorrowRecord, error) {
	if rec, ok := r.records[id]; ok {
		return rec, nil
	}
	return nil, apperrors.NotFound("borrow record")
}

func (r *fakeRepo) GetByPublicID(ctx context.Context, publicID string) (*BorrowRecord, error) {
	for _, rec := range r.records {
		if rec.PublicID == publicID {
			return rec, nil
		}
	}
	return nil, apperrors.NotFound("borrow record")
}

func (r *fakeRepo) GetByIdempotencyKey(ctx context.Context, userID uint64, key string) (*BorrowRecord, error) {
	for _, rec := range r.records {
		if rec.UserID == userID && rec.IdempotencyKey == key {
			return rec, nil
		}
	}
	return nil, apperrors.NotFound("borrow record")
}

func (r *fakeRepo) GetOpenByUserAndBook(ctx context.Context, userID, bookID uint64) (*BorrowRecord, error) {
	for _, rec := range r.records {
		if rec.UserID == userID && rec.BookID == bookID && rec.Status != StatusReturned {
			return rec, nil
		}
	}
	return nil, apperrors.NotFound("borrow record")
}

func (r *fakeRepo) GetAllByUserID(ctx context.Context, userID uint64, limit, offset int) ([]*BorrowRecord, int, error) {
	return nil, 0, nil
}

func (r *fakeRepo) GetAll(ctx context.Context, limit, offset int) ([]*BorrowRecord, int, error) {
	return nil, 0, nil
}

func (r *fakeRepo) HasOpenBorrow(ctx context.Context, userID, bookID uint64) (bool, error) {
	_, err := r.GetOpenByUserAndBook(ctx, userID, bookID)
	return err == nil, nil
}

func (r *fakeRepo) Create(ctx context.Context, record *BorrowRecord, event EventFor) (uint64, error) {
	if r.createErr != nil {
		return 0, r.createErr
	}
	r.nextID++
	record.ID = r.nextID
	record.Status = StatusActive
	record.BorrowedAt = time.Now()
	if _, err := event(record); err != nil {
		return 0, err
	}
	r.records[record.ID] = record
	return record.ID, nil
}

func (r *fakeRepo) MarkReturned(ctx context.Context, record *BorrowRecord, event EventFor) error {
	if r.markErr != nil {
		return r.markErr
	}
	now := time.Now()
	record.Status, record.ReturnedAt = StatusReturned, &now
	return nil
}

func (r *fakeRepo) MarkOverdue(ctx context.Context, event EventFor) (int, error) { return 0, nil }

func (r *fakeRepo) AddCompensation(ctx context.Context, key string, bookID uint64, cause error) error {
	r.compensations[key] = bookID
	return nil
}

func (r *fakeRepo) DueCompensations(ctx context.Context, limit int) ([]Compensation, error) {
	var out []Compensation
	for key, bookID := range r.compensations {
		out = append(out, Compensation{ReservationKey: key, BookID: bookID})
	}
	return out, nil
}

func (r *fakeRepo) CompensationFailed(ctx context.Context, key string, cause error) error { return nil }

func (r *fakeRepo) CompensationDone(ctx context.Context, key string) error {
	delete(r.compensations, key)
	return nil
}

var unavailable = &apperrors.AppError{Code: http.StatusServiceUnavailable, Message: "a dependent service is unavailable"}

// ── Tests ─────────────────────────────────────────────────

func TestBorrowHappyPath(t *testing.T) {
	cat, repo := newFakeCatalog(), newFakeRepo()
	svc := NewService(repo, cat, 14)

	resp, created, err := svc.Borrow(context.Background(), 7, "m@example.com", BorrowRequest{BookID: 3}, "key-1")
	if err != nil {
		t.Fatalf("borrow: %v", err)
	}
	if !created || resp.BookTitle != "Dune" || resp.Status != StatusActive {
		t.Fatalf("unexpected response: created=%v %+v", created, resp)
	}
	if len(cat.reserved) != 1 {
		t.Fatalf("expected one copy held, got %d", len(cat.reserved))
	}
}

func TestBorrowCompensatesWhenRecordCreationFails(t *testing.T) {
	cat, repo := newFakeCatalog(), newFakeRepo()
	repo.createErr = apperrors.Internal(errors.New("db down"))
	svc := NewService(repo, cat, 14)

	if _, _, err := svc.Borrow(context.Background(), 7, "m@example.com", BorrowRequest{BookID: 3}, "key-1"); err == nil {
		t.Fatal("expected error")
	}
	if len(cat.reserved) != 0 {
		t.Fatalf("copy leaked: %v", cat.reserved)
	}
	if cat.calls[len(cat.calls)-1] != "release" {
		t.Fatalf("expected a compensating release, calls=%v", cat.calls)
	}
}

func TestBorrowQueuesCompensationWhenCatalogUnreachable(t *testing.T) {
	cat, repo := newFakeCatalog(), newFakeRepo()
	cat.reserveErr = unavailable // outcome unknown
	cat.releaseErr = unavailable // and the inline release fails too
	svc := NewService(repo, cat, 14)

	if _, _, err := svc.Borrow(context.Background(), 7, "m@example.com", BorrowRequest{BookID: 3}, "key-1"); err == nil {
		t.Fatal("expected error")
	}
	if len(repo.compensations) != 1 {
		t.Fatalf("expected the release to be queued, got %v", repo.compensations)
	}

	// Catalog comes back: the compensator drains the queue.
	cat.releaseErr = nil
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	svc.RunCompensator(ctx, 10*time.Millisecond)
	if len(repo.compensations) != 0 {
		t.Fatalf("compensation not drained: %v", repo.compensations)
	}
}

func TestBorrowNoCopiesDoesNotCompensate(t *testing.T) {
	cat, repo := newFakeCatalog(), newFakeRepo()
	cat.reserveErr = grpcx.ErrNoCopies
	svc := NewService(repo, cat, 14)

	_, _, err := svc.Borrow(context.Background(), 7, "m@example.com", BorrowRequest{BookID: 3}, "key-1")
	if !errors.Is(err, grpcx.ErrNoCopies) {
		t.Fatalf("expected ErrNoCopies, got %v", err)
	}
	for _, c := range cat.calls {
		if c == "release" {
			t.Fatal("a definite 'no copies' answer must not trigger a release")
		}
	}
}

func TestBorrowIdempotentReplay(t *testing.T) {
	cat, repo := newFakeCatalog(), newFakeRepo()
	svc := NewService(repo, cat, 14)
	ctx := context.Background()

	first, created, err := svc.Borrow(ctx, 7, "m@example.com", BorrowRequest{BookID: 3}, "key-1")
	if err != nil || !created {
		t.Fatalf("first borrow: created=%v err=%v", created, err)
	}
	second, created, err := svc.Borrow(ctx, 7, "m@example.com", BorrowRequest{BookID: 3}, "key-1")
	if err != nil || created {
		t.Fatalf("replay: created=%v err=%v", created, err)
	}
	if first.ID != second.ID {
		t.Fatalf("replay returned a different record: %d vs %d", first.ID, second.ID)
	}
	reserves := 0
	for _, c := range cat.calls {
		if c == "reserve" {
			reserves++
		}
	}
	if reserves != 1 {
		t.Fatalf("expected exactly one reservation, got %d", reserves)
	}
}

func TestReturnReleasesBeforeMarking(t *testing.T) {
	cat, repo := newFakeCatalog(), newFakeRepo()
	svc := NewService(repo, cat, 14)
	ctx := context.Background()

	borrowed, _, err := svc.Borrow(ctx, 7, "m@example.com", BorrowRequest{BookID: 3}, "key-1")
	if err != nil {
		t.Fatal(err)
	}

	// Marking fails after the release: the member retries, and the second
	// release is a harmless no-op on the same key.
	repo.markErr = apperrors.Internal(errors.New("db blip"))
	if _, err := svc.Return(ctx, 7, "member", borrowed.ID); err == nil {
		t.Fatal("expected error")
	}
	repo.markErr = nil
	resp, err := svc.Return(ctx, 7, "member", borrowed.ID)
	if err != nil {
		t.Fatalf("retry return: %v", err)
	}
	if resp.Status != StatusReturned || len(cat.reserved) != 0 {
		t.Fatalf("unexpected state: status=%s held=%v", resp.Status, cat.reserved)
	}
}

func TestPublicIDIsUUIDv7AndAddressable(t *testing.T) {
	cat, repo := newFakeCatalog(), newFakeRepo()
	svc := NewService(repo, cat, 14)
	ctx := context.Background()

	b, _, err := svc.Borrow(ctx, 7, "m@example.com", BorrowRequest{BookID: 3}, "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.PublicID) != 36 || b.PublicID[14] != '7' {
		t.Fatalf("public_id %q is not a UUIDv7", b.PublicID)
	}
	if _, err := svc.ReturnByPublicID(ctx, 8, "member", b.PublicID); err == nil {
		t.Fatal("another member must not return it by public id either")
	}
	if resp, err := svc.ReturnByPublicID(ctx, 7, "member", b.PublicID); err != nil || resp.Status != StatusReturned {
		t.Fatalf("return by public id: %v", err)
	}
}

func TestReturnForbiddenForOtherMember(t *testing.T) {
	cat, repo := newFakeCatalog(), newFakeRepo()
	svc := NewService(repo, cat, 14)
	ctx := context.Background()

	borrowed, _, _ := svc.Borrow(ctx, 7, "m@example.com", BorrowRequest{BookID: 3}, "key-1")
	_, err := svc.Return(ctx, 8, "member", borrowed.ID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}
