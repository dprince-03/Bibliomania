package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveKeyGate(t *testing.T) {
	cases := []struct {
		keys    []string
		enabled bool
		blocked bool
	}{
		{[]string{"", ""}, false, false},
		{[]string{"sk_test_1", "sk_test_2"}, false, false},
		{[]string{"sk_live_1", ""}, false, true},
		{[]string{"", "sk_live_2"}, false, true}, // Paystack live key alone
		{[]string{"sk_live_1", "sk_live_2"}, true, false},
	}
	for _, c := range cases {
		err := CheckLiveKeyGate(c.enabled, c.keys...)
		if got := errors.Is(err, ErrLiveKeysBlocked); got != c.blocked {
			t.Errorf("keys=%v enabled=%v: blocked=%v, want %v", c.keys, c.enabled, got, c.blocked)
		}
	}
}

func TestRouterPicksProviderByCurrency(t *testing.T) {
	stripe := NewStripe("sk_test_x", "whsec")
	paystack := NewPaystack("sk_test_y", "http://unused", []string{"NGN", "GHS"})
	r := NewRouter(stripe, paystack)

	for currency, want := range map[string]string{"NGN": ProviderPaystack, "ghs": ProviderPaystack, "USD": ProviderStripe, "EUR": ProviderStripe} {
		p, err := r.For(currency, "")
		if err != nil || p.Name() != want {
			t.Errorf("%s: got %v %v, want %s", currency, p, err, want)
		}
	}
	if p, err := r.For("NGN", ProviderStripe); err != nil || p.Name() != ProviderStripe {
		t.Errorf("explicit stripe for NGN: %v %v", p, err)
	}
	if _, err := r.For("USD", ProviderPaystack); !errors.Is(err, errCurrencyUnsupported) {
		t.Errorf("paystack for USD should be unsupported, got %v", err)
	}

	// Only Paystack configured: USD has nowhere to go; NGN still works.
	onlyPaystack := NewRouter(nil, paystack)
	if _, err := onlyPaystack.For("USD", ""); !errors.Is(err, errCurrencyUnsupported) {
		t.Errorf("USD with only paystack: %v", err)
	}
	if _, err := onlyPaystack.For("NGN", ProviderStripe); !errors.Is(err, errProviderNotConfigured) {
		t.Errorf("stripe not configured: %v", err)
	}
	if _, err := NewRouter(nil, nil).For("NGN", ""); !errors.Is(err, errNotConfigured) {
		t.Errorf("nothing configured: %v", err)
	}
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// fakePaystack serves /transaction/initialize and /transaction/verify/{ref}.
func fakePaystack(t *testing.T, verified paystackTransaction) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk_test_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"status": false, "message": "Invalid key"})
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/transaction/initialize":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["amount"].(float64) != 250000 || body["currency"] != "NGN" || body["email"] != "reader@example.com" {
				t.Errorf("unexpected initialize body: %v", body)
			}
			json.NewEncoder(w).Encode(map[string]any{"status": true, "data": map[string]any{
				"authorization_url": "https://checkout.paystack.com/abc", "reference": body["reference"],
			}})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/transaction/verify/"):
			json.NewEncoder(w).Encode(map[string]any{"status": true, "data": verified})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestPaystackCheckoutAndWebhook(t *testing.T) {
	const secret = "sk_test_secret"
	srv := fakePaystack(t, paystackTransaction{ID: 42, Reference: "bib_7_x", Status: "success", Amount: 250000, Currency: "NGN"})
	defer srv.Close()
	p := NewPaystack(secret, srv.URL, []string{"NGN"})
	ctx := context.Background()

	ref, url, err := p.CreateCheckout(ctx, &Purchase{ID: 7, UserEmail: "reader@example.com", AmountCents: 250000, Currency: "NGN"}, "http://ok", "")
	if err != nil || url != "https://checkout.paystack.com/abc" || !strings.HasPrefix(ref, "bib_7_") {
		t.Fatalf("checkout: ref=%q url=%q err=%v", ref, url, err)
	}

	body := []byte(`{"event":"charge.success","data":{"id":42,"reference":"bib_7_x","status":"success","amount":1,"currency":"NGN"}}`)

	// Unsigned / wrongly signed → rejected.
	if _, err := p.ParseWebhook(ctx, body, http.Header{}); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("unsigned webhook: %v", err)
	}
	h := http.Header{}
	h.Set("x-paystack-signature", sign("wrong", body))
	if _, err := p.ParseWebhook(ctx, body, h); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("badly signed webhook: %v", err)
	}

	// Correctly signed: amount/status come from the Verify API, not the
	// (here deliberately wrong, amount 1) webhook body.
	h.Set("x-paystack-signature", sign(secret, body))
	ev, err := p.ParseWebhook(ctx, body, h)
	if err != nil {
		t.Fatalf("signed webhook: %v", err)
	}
	if ev.Status != StatusPaid || ev.AmountMinor != 250000 || ev.Currency != "NGN" || ev.Reference != "bib_7_x" || ev.ID != "charge.success:42" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	// Events we don't act on are ignored.
	other := []byte(`{"event":"transfer.success","data":{}}`)
	h.Set("x-paystack-signature", sign(secret, other))
	if ev, err := p.ParseWebhook(ctx, other, h); ev != nil || err != nil {
		t.Fatalf("ignored event: %+v %v", ev, err)
	}
}

func TestPaystackVerifyFailedIsNotPaid(t *testing.T) {
	const secret = "sk_test_secret"
	srv := fakePaystack(t, paystackTransaction{ID: 43, Reference: "bib_8_y", Status: "failed", Amount: 250000, Currency: "NGN"})
	defer srv.Close()
	p := NewPaystack(secret, srv.URL, []string{"NGN"})

	body := []byte(`{"event":"charge.success","data":{"id":43,"reference":"bib_8_y","status":"success"}}`)
	h := http.Header{}
	h.Set("x-paystack-signature", sign(secret, body))
	ev, err := p.ParseWebhook(context.Background(), body, h)
	if err != nil || ev.Status != StatusFailed {
		t.Fatalf("verify said failed, got %+v %v", ev, err)
	}
}
