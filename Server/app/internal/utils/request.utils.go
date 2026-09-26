package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"github.com/go-playground/validator/v10"
)

// MaxJSONBodyBytes caps every JSON request body. The API's largest real
// payloads (a book with a 2,000-char description, a bookmark note) are a
// few KB; 1 MB leaves ample headroom while stopping a client from making a
// service buffer an arbitrarily large body. File uploads are multipart and
// have their own limit (MAX_UPLOAD_SIZE_MB).
const MaxJSONBodyBytes = 1 << 20

// DecodeJSON strictly decodes a request body into dst:
//   - bodies over MaxJSONBodyBytes → 413;
//   - unknown fields → 400 (a typo like "titel" fails loudly instead of
//     being silently ignored);
//   - anything after the JSON value → 400.
//
// Returns *AppError, ready for HandleError.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &maxErr):
			return &apperrors.AppError{Code: http.StatusRequestEntityTooLarge,
				Message: fmt.Sprintf("request body must not exceed %d bytes", MaxJSONBodyBytes)}
		case errors.Is(err, io.EOF):
			return apperrors.BadRequest("request body is empty", nil)
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return apperrors.BadRequest("request body is not valid JSON", err)
		case errors.As(err, &typeErr):
			return apperrors.BadRequest(fmt.Sprintf("field %q has the wrong type", typeErr.Field), err)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			return apperrors.BadRequest(strings.TrimPrefix(err.Error(), "json: "), err)
		default:
			return apperrors.BadRequest("invalid request body", err)
		}
	}
	if dec.More() {
		return apperrors.BadRequest("request body must contain a single JSON object", nil)
	}
	return nil
}

// NewValidator returns the validator every service uses, with the custom
// rules registered:
//   - https_url: an absolute https:// URL with a host. Plain `url` would
//     also accept javascript:, data: and http: URLs, which become stored
//     XSS or mixed content once a frontend renders them as an <img src> or
//     link.
func NewValidator() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("https_url", func(fl validator.FieldLevel) bool {
		u, err := url.Parse(fl.Field().String())
		return err == nil && u.Scheme == "https" && u.Host != "" && len(fl.Field().String()) <= 2048
	})
	return v
}

// NormalizeEmail trims and lower-cases an address so "A@x.com " and
// "a@x.com" are the same account everywhere (auth's MySQL collation is
// case-insensitive, user-service's Postgres is not).
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
