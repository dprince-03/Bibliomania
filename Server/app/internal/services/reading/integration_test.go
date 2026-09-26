//go:build integration

package reading

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/testsupport"
)

type titleCatalog struct{}

func (titleCatalog) BookTitle(context.Context, uint64) (string, error) { return "Dune", nil }

// Offline devices syncing out of order, concurrently: whatever order the
// writes land in, the one with the latest client clock wins — including
// two writes microseconds apart (the old DATETIME(1s) collision).
func TestSyncLastWriteWinsUnderConcurrency(t *testing.T) {
	client := testsupport.MongoReplicaSet(t)
	db := client.Database("reading_test")
	if err := EnsureIndexes(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewStore(client, db, events.NewMongoOutbox(db)), titleCatalog{})
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	const n = 20
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			at := base.Add(time.Duration(i) * time.Microsecond) // all in the same second
			_, _ = svc.Sync(ctx, 7, 3, UpdateProgressRequest{CurrentPage: uint32(10 + i), TotalPages: 500, ClientUpdatedAt: &at})
		}()
	}
	close(start)
	wg.Wait()

	got, err := svc.GetSession(ctx, 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentPage != 10+n-1 {
		t.Fatalf("current_page=%d, want %d (the latest client write)", got.CurrentPage, 10+n-1)
	}

	// A stale device syncing later loses.
	old := base.Add(-time.Hour)
	if _, err := svc.Sync(ctx, 7, 3, UpdateProgressRequest{CurrentPage: 1, TotalPages: 500, ClientUpdatedAt: &old}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.GetSession(ctx, 7, 3); got.CurrentPage != 10+n-1 {
		t.Fatalf("stale write overwrote newer progress: page=%d", got.CurrentPage)
	}
}
