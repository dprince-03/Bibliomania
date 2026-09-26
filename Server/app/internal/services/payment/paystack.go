package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// paystackProvider takes payment through Paystack's hosted checkout
// (Transaction Initialize → authorization_url). Paystack is the natural
// processor for African currencies — NGN, GHS, ZAR, KES by default — where
// Stripe coverage is thin.
//
// API: https://paystack.com/docs/api/transaction/
type paystackProvider struct {
	secretKey  string
	baseURL    string
	currencies []string
	client     *http.Client
}

// NewPaystack returns nil when no secret key is configured. baseURL is
// normally https://api.paystack.co (overridable for tests).
func NewPaystack(secretKey, baseURL string, currencies []string) Provider {
	if secretKey == "" {
		return nil
	}
	for i, c := range currencies {
		currencies[i] = strings.ToUpper(strings.TrimSpace(c))
	}
	return &paystackProvider{
		secretKey:  secretKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		currencies: currencies,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *paystackProvider) Name() string         { return ProviderPaystack }
func (p *paystackProvider) Currencies() []string { return p.currencies }

type paystackEnvelope struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (p *paystackProvider) call(ctx context.Context, method, path string, body any, out any) error {
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	// baseURL is configuration, and every caller escapes its path segments
	// (url.PathEscape), so no request can leave the Paystack API host.
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader) // #nosec G704 -- fixed host from config, escaped path
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req) // #nosec G704 -- fixed host from config, escaped path
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var env paystackEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("paystack %s %s: HTTP %d, unreadable body", method, path, resp.StatusCode)
	}
	if resp.StatusCode/100 != 2 || !env.Status {
		return fmt.Errorf("paystack %s %s: HTTP %d: %s", method, path, resp.StatusCode, env.Message)
	}
	if out != nil {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

// CreateCheckout initialises a transaction. The reference is ours (not
// Paystack's) so it's known before the call and unique per attempt.
func (p *paystackProvider) CreateCheckout(ctx context.Context, pur *Purchase, successURL, _ string) (string, string, error) {
	reference, err := newReference(pur.ID)
	if err != nil {
		return "", "", err
	}
	var data struct {
		AuthorizationURL string `json:"authorization_url"`
		Reference        string `json:"reference"`
	}
	err = p.call(ctx, http.MethodPost, "/transaction/initialize", map[string]any{
		"email":        pur.UserEmail,
		"amount":       pur.AmountCents, // subunits (kobo, pesewas, cents) — same as price_cents
		"currency":     pur.Currency,
		"reference":    reference,
		"callback_url": successURL,
		"metadata": map[string]any{
			"purchase_id": pur.ID,
			"book_id":     pur.BookID,
		},
	}, &data)
	if err != nil {
		return "", "", err
	}
	return data.Reference, data.AuthorizationURL, nil
}

type paystackTransaction struct {
	ID        int64  `json:"id"`
	Reference string `json:"reference"`
	Status    string `json:"status"` // success | failed | abandoned | ...
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

// ParseWebhook authenticates with x-paystack-signature (HMAC-SHA512 of the
// raw body, keyed with the secret key), then — as Paystack recommends —
// re-reads the transaction from the Verify API rather than trusting the
// webhook body's status and amount.
func (p *paystackProvider) ParseWebhook(ctx context.Context, payload []byte, headers http.Header) (*WebhookEvent, error) {
	if !validPaystackSignature(p.secretKey, payload, headers.Get("x-paystack-signature")) {
		return nil, ErrBadSignature
	}

	var event struct {
		Event string              `json:"event"`
		Data  paystackTransaction `json:"data"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	if event.Event != "charge.success" {
		return nil, nil
	}

	var tx paystackTransaction
	if err := p.call(ctx, http.MethodGet, "/transaction/verify/"+url.PathEscape(event.Data.Reference), nil, &tx); err != nil {
		return nil, fmt.Errorf("verifying paystack transaction: %w", err)
	}

	status := StatusFailed
	if tx.Status == "success" {
		status = StatusPaid
	}
	return &WebhookEvent{
		// Paystack webhooks carry no event ID; the transaction ID + event
		// name is unique per outcome and stable across redeliveries.
		ID:          fmt.Sprintf("%s:%d", event.Event, tx.ID),
		Type:        event.Event,
		Reference:   tx.Reference,
		PaymentID:   fmt.Sprint(tx.ID),
		Status:      status,
		AmountMinor: tx.Amount,
		Currency:    strings.ToUpper(tx.Currency),
	}, nil
}

func validPaystackSignature(secret string, payload []byte, signature string) bool {
	if signature == "" {
		return false
	}
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(signature)))
}

// newReference: "bib_<purchase id>_<random>" — Paystack references must be
// unique per transaction and may only contain [A-Za-z0-9-_.=].
func newReference(purchaseID uint64) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("bib_%d_%s", purchaseID, hex.EncodeToString(b)), nil
}
