package apiversion

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, r *Registry, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot) // "reached the handler"
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestKnownVersionPassesWithHeader(t *testing.T) {
	r, _ := NewRegistry([]string{"v1"}, "")
	rec := serve(t, r, "/api/v1/books")
	if rec.Code != http.StatusTeapot || rec.Header().Get("API-Version") != "v1" {
		t.Fatalf("code=%d api-version=%q", rec.Code, rec.Header().Get("API-Version"))
	}
	if rec.Header().Get("Deprecation") != "" {
		t.Fatal("current version must not be marked deprecated")
	}
}

func TestUnknownVersionRejected(t *testing.T) {
	r, _ := NewRegistry([]string{"v1"}, "")
	for _, path := range []string{"/api/v2/books", "/api/books", "/api/"} {
		if rec := serve(t, r, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: code=%d, want 404", path, rec.Code)
		}
	}
}

func TestNonAPIPathsPassThrough(t *testing.T) {
	r, _ := NewRegistry([]string{"v1"}, "")
	for _, path := range []string{"/graphql", "/health", "/api/versions"} {
		if rec := serve(t, r, path); rec.Code != http.StatusTeapot {
			t.Errorf("%s: code=%d, want pass-through", path, rec.Code)
		}
	}
}

func TestDeprecatedVersionHeaders(t *testing.T) {
	r, err := NewRegistry([]string{"v1", "v2"}, "v1:2027-01-01:2027-07-01")
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, r, "/api/v1/books")
	if rec.Header().Get("Deprecation") != "@1798761600" {
		t.Errorf("Deprecation = %q", rec.Header().Get("Deprecation"))
	}
	if rec.Header().Get("Sunset") != "Thu, 01 Jul 2027 00:00:00 GMT" {
		t.Errorf("Sunset = %q", rec.Header().Get("Sunset"))
	}
	if rec := serve(t, r, "/api/v2/books"); rec.Header().Get("Deprecation") != "" {
		t.Error("v2 is current and must not be deprecated")
	}
}

func TestBadDeprecationConfig(t *testing.T) {
	for _, cfg := range []string{"v9:2027-01-01", "v1", "v1:not-a-date"} {
		if _, err := NewRegistry([]string{"v1"}, cfg); err == nil {
			t.Errorf("%q: expected error", cfg)
		}
	}
}
