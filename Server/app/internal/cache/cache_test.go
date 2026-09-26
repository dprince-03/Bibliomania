package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type book struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func newCache(t *testing.T) (Cache, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	return NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "t"), mr
}

func TestReadThroughHitsCache(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	loads := 0
	load := func() (book, error) { loads++; return book{1, "Dune"}, nil }

	for i := 0; i < 3; i++ {
		b, err := ReadThrough(ctx, c, "b1", time.Minute, load)
		if err != nil || b.Title != "Dune" {
			t.Fatalf("got %+v %v", b, err)
		}
	}
	if loads != 1 {
		t.Fatalf("database loaded %d times, want 1 (cache must hit)", loads)
	}
}

func TestReadThroughServesStaleWhenDatabaseDown(t *testing.T) {
	c, mr := newCache(t)
	ctx := context.Background()
	_, _ = ReadThrough(ctx, c, "b1", time.Minute, func() (book, error) { return book{1, "Dune"}, nil })

	mr.FastForward(2 * time.Minute) // fresh entry expires, stale copy remains
	b, err := ReadThrough(ctx, c, "b1", time.Minute, func() (book, error) {
		return book{}, apperrors.Internal(errors.New("connection refused"))
	})
	if err != nil || b.Title != "Dune" {
		t.Fatalf("want stale copy, got %+v %v", b, err)
	}

	// A 404 is a real answer, never papered over with stale data.
	_, err = ReadThrough(ctx, c, "b1", time.Minute, func() (book, error) { return book{}, apperrors.NotFound("book") })
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Fatalf("want 404 passthrough, got %v", err)
	}
}
