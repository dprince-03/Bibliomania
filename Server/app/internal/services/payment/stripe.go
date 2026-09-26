package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

// stripeProvider takes payment through Stripe Checkout. It accepts any
// currency (Stripe decides at checkout time), so it's the fallback for
// everything Paystack doesn't handle.
type stripeProvider struct {
	client        *stripe.Client
	webhookSecret string
}

// NewStripe returns nil when no secret key is configured.
func NewStripe(secretKey, webhookSecret string) Provider {
	if secretKey == "" {
		return nil
	}
	return &stripeProvider{client: stripe.NewClient(secretKey), webhookSecret: webhookSecret}
}

func (s *stripeProvider) Name() string         { return ProviderStripe }
func (s *stripeProvider) Currencies() []string { return nil }

func (s *stripeProvider) CreateCheckout(ctx context.Context, p *Purchase, successURL, cancelURL string) (string, string, error) {
	ref := utils.Uint64Key(p.ID)
	session, err := s.client.V1CheckoutSessions.Create(ctx, &stripe.CheckoutSessionCreateParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		CustomerEmail:     stripe.String(p.UserEmail),
		ClientReferenceID: stripe.String(ref),
		Metadata:          map[string]string{"purchase_id": ref, "book_id": utils.Uint64Key(p.BookID)},
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{{
			Quantity: stripe.Int64(1),
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:    stripe.String(strings.ToLower(p.Currency)),
				UnitAmount:  stripe.Int64(p.AmountCents),
				ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{Name: stripe.String(p.BookTitle)},
			},
		}},
	})
	if err != nil {
		return "", "", err
	}
	return session.ID, session.URL, nil
}

func (s *stripeProvider) ParseWebhook(_ context.Context, payload []byte, headers http.Header) (*WebhookEvent, error) {
	if s.webhookSecret == "" {
		return nil, errNotConfigured
	}
	event, err := webhook.ConstructEvent(payload, headers.Get("Stripe-Signature"), s.webhookSecret)
	if err != nil {
		return nil, ErrBadSignature
	}

	var status string
	switch event.Type {
	case "checkout.session.completed":
		status = StatusPaid
	case "checkout.session.expired", "checkout.session.async_payment_failed":
		status = StatusFailed
	default:
		return nil, nil
	}

	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return nil, err
	}
	if status == StatusPaid && session.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		// Completed but not yet paid (delayed payment methods) — wait for
		// checkout.session.async_payment_succeeded. Not handled yet.
		return nil, nil
	}

	out := &WebhookEvent{
		ID:          event.ID,
		Type:        string(event.Type),
		Reference:   session.ID,
		Status:      status,
		AmountMinor: session.AmountTotal,
		Currency:    strings.ToUpper(string(session.Currency)),
	}
	if session.PaymentIntent != nil {
		out.PaymentID = session.PaymentIntent.ID
	}
	return out, nil
}
