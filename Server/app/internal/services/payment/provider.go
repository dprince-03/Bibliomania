package payment

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
)

// Provider is one payment processor. payment-service supports Stripe and
// Paystack side by side; which one takes a purchase is decided by the
// book's currency (see Router), or explicitly by the client.
type Provider interface {
	Name() string
	// Currencies this provider accepts (upper-case ISO 4217); nil = any.
	Currencies() []string
	// CreateCheckout starts a hosted checkout for p. reference is what
	// identifies the payment in later webhooks (stored as
	// book_purchases.provider_reference); url is where to send the reader.
	CreateCheckout(ctx context.Context, p *Purchase, successURL, cancelURL string) (reference, url string, err error)
	// ParseWebhook authenticates a webhook delivery and turns it into a
	// provider-neutral event. It returns ErrBadSignature for anything not
	// provably from the provider, and (nil, nil) for event types we ignore.
	ParseWebhook(ctx context.Context, payload []byte, headers http.Header) (*WebhookEvent, error)
}

// WebhookEvent is a provider-neutral payment outcome.
type WebhookEvent struct {
	// ID dedupes redelivered webhooks (webhook_events primary key, with
	// the provider name).
	ID        string
	Type      string
	Reference string // matches book_purchases.provider_reference
	PaymentID string // provider's own payment/transaction ID
	Status    string // StatusPaid or StatusFailed
	// What was actually charged — checked against the purchase before it's
	// marked paid, so a tampered or mismatched amount never grants a book.
	AmountMinor int64
	Currency    string
}

const (
	ProviderStripe   = "stripe"
	ProviderPaystack = "paystack"
)

var ErrBadSignature = errors.New("invalid webhook signature")

// Router picks a provider for a purchase.
type Router struct {
	providers map[string]Provider
	// Order of preference when a currency is supported by several: the
	// currency-specific provider (Paystack for NGN/GHS/ZAR/KES) wins over
	// one that accepts anything (Stripe).
	order []string
}

// NewRouter registers the configured providers (nil entries are skipped —
// a provider without keys simply isn't offered).
func NewRouter(providers ...Provider) *Router {
	r := &Router{providers: map[string]Provider{}}
	for _, p := range providers {
		if p == nil {
			continue
		}
		r.providers[p.Name()] = p
		if p.Currencies() != nil {
			r.order = append([]string{p.Name()}, r.order...) // specific first
		} else {
			r.order = append(r.order, p.Name())
		}
	}
	return r
}

// Get returns a configured provider by name.
func (r *Router) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// For picks the provider for currency: the explicitly requested one (if
// configured and it accepts the currency), else the first configured one
// that accepts it.
func (r *Router) For(currency, requested string) (Provider, error) {
	currency = strings.ToUpper(currency)
	if requested != "" {
		p, ok := r.providers[requested]
		if !ok {
			return nil, errProviderNotConfigured
		}
		if !accepts(p, currency) {
			return nil, errCurrencyUnsupported
		}
		return p, nil
	}
	for _, name := range r.order {
		if p := r.providers[name]; accepts(p, currency) {
			return p, nil
		}
	}
	if len(r.providers) == 0 {
		return nil, errNotConfigured
	}
	return nil, errCurrencyUnsupported
}

func accepts(p Provider, currency string) bool {
	return p.Currencies() == nil || slices.Contains(p.Currencies(), currency)
}
