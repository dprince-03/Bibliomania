//go:build integration

package catalog

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/dprince-03/Bibliomania/internal/cache"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/testsupport"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newIntegrationBookService(t *testing.T) (*BookService, func(total int) uint64) {
	t.Helper()
	db := testsupport.Postgres(t, Migrations)
	mr := miniredis.RunT(t)
	svc := NewBookService(db, NewBookRepository(db), NewAuthorRepository(db), NewBookAuthorRepository(db),
		cache.NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test"), nil, nil, 10)
	n := 0
	insert := func(total int) uint64 {
		n++
		var id uint64
		if err := db.Get(&id, `INSERT INTO books (title, isbn, genre, total_copies, available_copies)
			VALUES ($1, $2, 'test', $3, $3) RETURNING id`, fmt.Sprintf("Book %d", n), fmt.Sprintf("isbn-%d", n), total); err != nil {
			t.Fatal(err)
		}
		return id
	}
	return svc, insert
}

// More borrowers than copies, all at once: exactly `total` reservations
// succeed, the rest get ErrNoCopies, and available_copies lands on 0 —
// never negative.
func TestReserveCopyNeverOversells(t *testing.T) {
	svc, insert := newIntegrationBookService(t)
	ctx := context.Background()
	const total, borrowers = 5, 40
	bookID := insert(total)

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, noCopies := 0, 0
	start := make(chan struct{})
	for i := range borrowers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			reserved, err := svc.ReserveCopy(ctx, bookID, fmt.Sprintf("k-%d", i))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && reserved:
				ok++
			case errors.Is(err, grpcx.ErrNoCopies):
				noCopies++
			default:
				t.Errorf("unexpected: reserved=%v err=%v", reserved, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if ok != total || noCopies != borrowers-total {
		t.Fatalf("ok=%d noCopies=%d, want %d/%d", ok, noCopies, total, borrowers-total)
	}
	b, err := svc.bookRepo.GetByID(ctx, bookID)
	if err != nil {
		t.Fatal(err)
	}
	if b.AvailableCopies != 0 {
		t.Fatalf("available_copies=%d, want 0", b.AvailableCopies)
	}
}

// A retried reservation (same key) takes one copy, not two; releasing
// twice gives one back, not two.
func TestReserveReleaseIdempotent(t *testing.T) {
	svc, insert := newIntegrationBookService(t)
	ctx := context.Background()
	bookID := insert(3)

	for range 3 {
		if _, err := svc.ReserveCopy(ctx, bookID, "same-key"); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := svc.bookRepo.GetByID(ctx, bookID); b.AvailableCopies != 2 {
		t.Fatalf("after 3 retries of one reservation: available=%d, want 2", b.AvailableCopies)
	}
	for range 3 {
		if _, err := svc.ReleaseCopy(ctx, "same-key"); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := svc.bookRepo.GetByID(ctx, bookID); b.AvailableCopies != 3 {
		t.Fatalf("after 3 releases: available=%d, want 3", b.AvailableCopies)
	}
}

// Librarian edits total_copies while members borrow. Whatever interleaving
// happens, the invariant holds: available = total − copies on loan, and
// 0 ≤ available ≤ total. (Before the relative-delta fix, Update wrote an
// absolute available_copies computed from a stale read, losing reservations.)
func TestUpdateRacingReservationsKeepsInvariant(t *testing.T) {
	svc, insert := newIntegrationBookService(t)
	ctx := context.Background()
	bookID := insert(10)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range 15 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = svc.ReserveCopy(ctx, bookID, fmt.Sprintf("r-%d", i))
		}()
	}
	for _, total := range []int{12, 8, 15, 11, 20} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.Update(ctx, bookID, UpdateBookRequest{TotalCopies: &total})
			if err != nil && !errors.Is(err, ErrCopiesOnLoan) {
				t.Errorf("update to %d: %v", total, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	b, err := svc.bookRepo.GetByID(ctx, bookID)
	if err != nil {
		t.Fatal(err)
	}
	var onLoan int
	if err := svc.db.Get(&onLoan, `SELECT count(*) FROM copy_reservations WHERE book_id = $1 AND status = 'reserved'`, bookID); err != nil {
		t.Fatal(err)
	}
	if b.AvailableCopies != b.TotalCopies-onLoan || b.AvailableCopies < 0 || b.AvailableCopies > b.TotalCopies {
		t.Fatalf("invariant broken: total=%d available=%d on_loan=%d", b.TotalCopies, b.AvailableCopies, onLoan)
	}
}
