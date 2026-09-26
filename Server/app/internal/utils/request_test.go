package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
)

type sample struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func decode(body string) error {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	var s sample
	return DecodeJSON(httptest.NewRecorder(), r, &s)
}

func code(err error) int {
	if appErr, ok := err.(*apperrors.AppError); ok {
		return appErr.Code
	}
	return 0
}

func TestDecodeJSON(t *testing.T) {
	cases := map[string]int{
		`{"name":"a","count":1}`:   0,
		`{"name":"a","extra":1}`:   400, // unknown field
		`{"name":"a"}{"name":"b"}`: 400, // trailing object
		`{"count":"x"}`:            400, // wrong type
		`{"name":`:                 400, // truncated
		``:                         400, // empty
		`{"name":"` + strings.Repeat("a", 2<<20) + `"}`: 413,
	}
	for body, want := range cases {
		label := body
		if len(label) > 40 {
			label = label[:40] + "…"
		}
		if got := code(decode(body)); got != want {
			t.Errorf("%q: got %d, want %d", label, got, want)
		}
	}
}

func TestHTTPSURLValidator(t *testing.T) {
	v := NewValidator()
	type s struct {
		URL string `validate:"omitempty,https_url"`
	}
	for url, ok := range map[string]bool{
		"https://example.com/a.png": true,
		"":                          true, // omitempty
		"http://example.com/a.png":  false,
		"javascript:alert(1)":       false,
		"data:image/png;base64,AA":  false,
		"https://":                  false,
		"//example.com/a.png":       false,
	} {
		if err := v.Struct(s{URL: url}); (err == nil) != ok {
			t.Errorf("%q: valid=%v, want %v", url, err == nil, ok)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Ada@Example.COM "); got != "ada@example.com" {
		t.Fatal(got)
	}
}
