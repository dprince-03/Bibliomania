//go:build integration

package payment

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	"github.com/dprince-03/Bibliomania/internal/testsupport"

	"google.golang.org/grpc"
)

type priceCatalog struct{ catalogv1.CatalogServiceClient }

func (priceCatalog) GetBook(ctx context.Context, in *catalogv1.GetBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
	price, cur := int64(1500), "USD"
	return &catalogv1.Book{Id: in.GetBookId(), Title: "Dune", PriceCents: &price, Currency: &cur}, nil
}

type countingProvider struct{ calls atomic.Int32 }

func (*countingProvider) Name() string         { return ProviderStripe } // provider column is CHECK-constrained
func (*countingProvider) Currencies() []string { return nil }
func (p *countingProvider) CreateCheckout(ctx context.Context, pu *Purchase, _, _ string) (string, string, error) {
	p.calls.Add(1)
	time.Sleep(50 * time.Millisecond) // a real provider round trip
	return "ref-" + pu.PublicID, "https://pay.example/" + pu.PublicID, nil
}
func (*countingProvider) ParseWebhook(context.Context, []byte, http.Header) (*WebhookEvent, error) {
	return nil, nil
}

// A double-click / two tabs / a retry storm: many concurrent checkouts for
// the same user+book must open ONE provider session and ONE pending row;
// the rest either get that same checkout back or a retryable 409.
func TestConcurrentCheckoutOpensOnePending(t *testing.T) {
	db := testsupport.Postgres(t, Migrations)
	provider := &countingProvider{}
	svc := NewService(NewRepository(db), priceCatalog{}, NewRouter(provider), "https://ok", "https://cancel")
	ctx := context.Background()

	const n = 15
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := svc.CreateCheckout(ctx, 7, "m@example.com", 3, ""); err != nil {
				t.Log(err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if c := provider.calls.Load(); c != 1 {
		t.Fatalf("provider sessions opened: %d, want 1", c)
	}
	var pending int
	if err := db.Get(&pending, `SELECT count(*) FROM book_purchases WHERE user_id = 7 AND book_id = 3 AND status = 'pending'`); err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("pending purchases: %d, want 1", pending)
	}

	// Once the first checkout is fully started, a retry gets the same one.
	a, err := svc.CreateCheckout(ctx, 7, "m@example.com", 3, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateCheckout(ctx, 7, "m@example.com", 3, "")
	if err != nil || a.PublicID != b.PublicID || a.CheckoutURL != b.CheckoutURL {
		t.Fatalf("retry should return the same checkout: %+v vs %+v (%v)", a, b, err)
	}
	if len(a.PublicID) != 36 {
		t.Fatalf("public id %q is not a UUID", a.PublicID)
	}
}
