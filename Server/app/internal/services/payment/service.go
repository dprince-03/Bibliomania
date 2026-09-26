package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/utils"
)

const serviceName = "payment-service"

// pendingCheckoutTTL: how long an open checkout is handed back to repeat
// requests. Stripe Checkout sessions expire after 24h; stay under it.
const pendingCheckoutTTL = 23 * time.Hour

// ErrLiveKeysBlocked stops payment-service from starting with live keys
// (Stripe or Paystack) before Step 38's penetration test has passed.
var ErrLiveKeysBlocked = fmt.Errorf("live payment keys are blocked until PAYMENTS_LIVE_ENABLED=true (Step 38 launch gate)")

// CheckLiveKeyGate enforces Step 38: payments may be built and tested
// with test keys, but must not take real money until explicitly enabled.
// Both Stripe and Paystack prefix live secret keys with "sk_live_".
func CheckLiveKeyGate(liveEnabled bool, secretKeys ...string) error {
	if liveEnabled {
		return nil
	}
	for _, k := range secretKeys {
		if strings.HasPrefix(k, "sk_live_") {
			return ErrLiveKeysBlocked
		}
	}
	return nil
}

var (
	errNotConfigured          = &apperrors.AppError{Code: http.StatusServiceUnavailable, Message: "payments are not configured on this server"}
	errProviderNotConfigured  = &apperrors.AppError{Code: http.StatusServiceUnavailable, Message: "that payment provider is not configured on this server"}
	errCurrencyUnsupported    = apperrors.BadRequest("no configured payment provider accepts this book's currency", nil)
	errUnknownWebhookProvider = &apperrors.AppError{Code: http.StatusNotFound, Message: "unknown payment provider"}
)

type Service struct {
	repo       Repository
	catalog    catalogv1.CatalogServiceClient
	providers  *Router
	successURL string
	cancelURL  string
}

func NewService(repo Repository, catalog catalogv1.CatalogServiceClient, providers *Router, successURL, cancelURL string) *Service {
	return &Service{
		repo:       repo,
		catalog:    catalog,
		providers:  providers,
		successURL: successURL,
		cancelURL:  cancelURL,
	}
}

// CreateCheckout starts a hosted checkout for one book with the provider
// that takes its currency (or the one the client asked for).
func (s *Service) CreateCheckout(ctx context.Context, userID uint64, email string, bookID uint64, requestedProvider string) (*CheckoutResponse, error) {
	book, err := s.catalog.GetBook(ctx, &catalogv1.GetBookRequest{BookId: bookID})
	if err != nil {
		return nil, grpcx.FromStatus(err)
	}
	if book.PriceCents == nil || book.Currency == nil {
		return nil, apperrors.BadRequest("this book is not for sale", nil)
	}
	currency := strings.ToUpper(book.GetCurrency())

	provider, err := s.providers.For(currency, requestedProvider)
	if err != nil {
		return nil, err
	}

	owned, err := s.repo.HasPaid(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	if owned {
		return nil, apperrors.Conflict("you already own this book")
	}

	purchase := &Purchase{
		PublicID:    utils.NewPublicID(),
		UserID:      userID,
		UserEmail:   email,
		BookID:      bookID,
		BookTitle:   book.GetTitle(),
		AmountCents: book.GetPriceCents(),
		Currency:    currency,
		Provider:    provider.Name(),
	}
	if err := s.repo.Create(ctx, purchase); err != nil {
		if !errors.Is(err, ErrPendingExists) {
			return nil, err
		}
		// A checkout for this book is already open (a retry, a double
		// click, a second tab): hand back the same one rather than a
		// second payment. Stale ones (older than the provider's session
		// lifetime) are closed and replaced.
		existing, gerr := s.repo.GetPending(ctx, userID, bookID)
		if gerr != nil {
			return nil, gerr
		}
		if existing.CheckoutURL != nil && time.Since(existing.CreatedAt) < pendingCheckoutTTL && existing.Provider == provider.Name() {
			return &CheckoutResponse{PurchaseID: existing.ID, PublicID: existing.PublicID, Provider: existing.Provider, CheckoutURL: *existing.CheckoutURL}, nil
		}
		if existing.CheckoutURL == nil && time.Since(existing.CreatedAt) < time.Minute {
			// The other request is still talking to the provider.
			return nil, apperrors.Conflict("a checkout for this book is already being started — retry in a moment")
		}
		_ = s.repo.MarkFailed(ctx, existing.ID)
		if err := s.repo.Create(ctx, purchase); err != nil {
			if errors.Is(err, ErrPendingExists) {
				return nil, apperrors.Conflict("a checkout for this book is already being started — retry in a moment")
			}
			return nil, err
		}
	}

	reference, url, err := provider.CreateCheckout(ctx, purchase, s.successURL, s.cancelURL)
	if err != nil {
		_ = s.repo.MarkFailed(ctx, purchase.ID)
		slog.ErrorContext(ctx, "starting checkout failed", "provider", provider.Name(), "purchase_id", purchase.ID, "error", err)
		return nil, &apperrors.AppError{Code: http.StatusBadGateway, Message: "could not start checkout", Err: err}
	}
	if err := s.repo.SetReference(ctx, purchase.ID, reference, url); err != nil {
		return nil, err
	}

	return &CheckoutResponse{PurchaseID: purchase.ID, PublicID: purchase.PublicID, Provider: provider.Name(), CheckoutURL: url}, nil
}

func (s *Service) ListMine(ctx context.Context, userID uint64, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	purchases, total, err := s.repo.ListByUser(ctx, userID, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}
	items := make([]PurchaseResponse, len(purchases))
	for i, p := range purchases {
		items[i] = mapPurchase(p)
	}
	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)
	return &resp, nil
}

// HandleWebhook authenticates a provider's webhook and settles the
// purchase it refers to. Providers retry until they get a 2xx, so every
// outcome other than "not provably from the provider" returns nil —
// problems needing a human (amount mismatch, double purchase) are logged.
func (s *Service) HandleWebhook(ctx context.Context, providerName string, payload []byte, headers http.Header) error {
	provider, ok := s.providers.Get(providerName)
	if !ok {
		if providerName == ProviderStripe || providerName == ProviderPaystack {
			return errProviderNotConfigured
		}
		return errUnknownWebhookProvider
	}

	ev, err := provider.ParseWebhook(ctx, payload, headers)
	if errors.Is(err, ErrBadSignature) {
		return apperrors.BadRequest("invalid webhook signature", nil)
	}
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return err
		}
		// e.g. Paystack's verify call failed — let the provider retry.
		slog.ErrorContext(ctx, "processing webhook failed", "provider", providerName, "error", err)
		return &apperrors.AppError{Code: http.StatusBadGateway, Message: "webhook processing failed, retry", Err: err}
	}
	if ev == nil {
		return nil // an event type we don't act on
	}

	eventType := events.TypeBookPurchased
	if ev.Status == StatusFailed {
		eventType = events.TypePaymentFailed
	}

	settled, err := s.repo.SettleFromWebhook(ctx, providerName, ev, func(p *Purchase) (events.Event, error) {
		return events.New(ctx, eventType, serviceName, utils.Uint64Key(p.ID), events.PaymentEvent{
			PurchaseID:  p.ID,
			UserID:      p.UserID,
			UserEmail:   p.UserEmail,
			BookID:      p.BookID,
			BookTitle:   p.BookTitle,
			AmountCents: p.AmountCents,
			Currency:    p.Currency,
			Provider:    p.Provider,
		})
	})
	if err != nil {
		slog.ErrorContext(ctx, "settling purchase failed — needs manual review (refund?)",
			"provider", providerName, "reference", ev.Reference, "error", err)
		return nil
	}
	if settled != nil {
		slog.InfoContext(ctx, "purchase settled", "provider", providerName, "purchase_id", settled.ID, "status", settled.Status)
	}
	return nil
}

func mapPurchase(p *Purchase) PurchaseResponse {
	return PurchaseResponse{
		ID:          p.ID,
		PublicID:    p.PublicID,
		BookID:      p.BookID,
		BookTitle:   p.BookTitle,
		AmountCents: p.AmountCents,
		Currency:    p.Currency,
		Provider:    p.Provider,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt,
	}
}
