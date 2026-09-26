package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A relayed "100 Continue" must not swallow the final status. Checked over
// a real HTTP connection: that's where the bug showed (the gateway answered
// 200 with a 413 body for large uploads).
func TestLoggerKeepsStatusAfterInformational(t *testing.T) {
	srv := httptest.NewServer(Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusContinue)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte("too big"))
	})))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}
