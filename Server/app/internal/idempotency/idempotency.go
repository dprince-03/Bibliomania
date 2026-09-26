// Package idempotency makes any mutating REST request retry-safe at the
// gateway, following the IETF "Idempotency-Key" HTTP header draft
// (draft-ietf-httpapi-idempotency-key-header):
//
//   - A client sends `Idempotency-Key: <unique value>` on a POST/PUT/PATCH/
//     DELETE. The first request runs; its response is stored in Redis for
//     24h. A retry with the same key gets that stored response back
//     (with `Idempotent-Replayed: true`) instead of running again — a
//     timed-out "create bookmark" or "checkout" can't happen twice.
//   - Same key + a different body → 422 (the key was reused by mistake).
//   - Same key while the first request is still running → 409.
//   - 5xx responses aren't stored: the operation may not have happened, so
//     a retry must be allowed to run.
//
// Keys are scoped per caller (JWT subject, or "anon") + method + path, so
// two users can't collide or replay each other's responses. Multipart
// uploads are not covered (bodies too large to fingerprint/store).
// Redis being down fails open: the request runs, unprotected, and it's
// logged — availability over strictness, since services with their own
// idempotency (borrowing, payments) still protect themselves.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/redis/go-redis/v9"
)

const (
	Header         = "Idempotency-Key"
	ReplayedHeader = "Idempotent-Replayed"
	maxKeyLen      = 100
	maxStoredBody  = 1 << 20
	lockTTL        = 60 * time.Second
	resultTTL      = 24 * time.Hour
)

// CallerFunc identifies the caller for key scoping (e.g. the JWT subject).
type CallerFunc func(r *http.Request) string

type record struct {
	State       string              `json:"state"` // "running" | "done"
	Fingerprint string              `json:"fingerprint"`
	Status      int                 `json:"status,omitempty"`
	Header      map[string][]string `json:"header,omitempty"`
	Body        []byte              `json:"body,omitempty"`
}

type Middleware struct {
	rdb    *redis.Client
	caller CallerFunc
	prefix string
}

func New(rdb *redis.Client, caller CallerFunc) *Middleware {
	return &Middleware{rdb: rdb, caller: caller, prefix: "gw:idem:"}
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(Header)
		if key == "" || !mutating(r.Method) || strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			next.ServeHTTP(w, r)
			return
		}
		if len(key) > maxKeyLen {
			utils.Error(w, apperrors.BadRequest("Idempotency-Key must be at most 100 characters", nil))
			return
		}

		// Fingerprint the body, then give the handler a fresh reader.
		body, err := io.ReadAll(io.LimitReader(r.Body, utils.MaxJSONBodyBytes+1))
		if err != nil {
			utils.Error(w, apperrors.BadRequest("unreadable request body", err))
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		fingerprint := hash(r.Method, r.URL.RequestURI(), string(body))
		redisKey := m.prefix + hash(m.caller(r), r.Method, r.URL.Path, key)
		ctx := r.Context()

		lock, _ := json.Marshal(record{State: "running", Fingerprint: fingerprint})
		acquired, err := m.rdb.SetNX(ctx, redisKey, lock, lockTTL).Result()
		if err != nil {
			slog.WarnContext(ctx, "idempotency store unavailable — request runs unprotected", "error", err)
			next.ServeHTTP(w, r)
			return
		}
		if !acquired {
			m.replay(ctx, w, redisKey, fingerprint)
			return
		}

		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// Store the outcome — or release the key so a retry can run.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if rec.status >= 500 || rec.overflow {
			m.rdb.Del(saveCtx, redisKey)
			return
		}
		done, _ := json.Marshal(record{
			State: "done", Fingerprint: fingerprint,
			Status: rec.status, Header: storedHeaders(w.Header()), Body: rec.body.Bytes(),
		})
		if err := m.rdb.Set(saveCtx, redisKey, done, resultTTL).Err(); err != nil {
			slog.WarnContext(ctx, "could not store idempotent response", "error", err)
		}
	})
}

func (m *Middleware) replay(ctx context.Context, w http.ResponseWriter, redisKey, fingerprint string) {
	raw, err := m.rdb.Get(ctx, redisKey).Bytes()
	if err != nil {
		// Expired between SETNX and GET — vanishingly rare; ask for a retry.
		utils.Error(w, apperrors.Conflict("request with this Idempotency-Key is being processed, retry shortly"))
		return
	}
	var rec record
	if err := json.Unmarshal(raw, &rec); err != nil {
		utils.Error(w, apperrors.Internal(err))
		return
	}
	if rec.Fingerprint != fingerprint {
		utils.Error(w, apperrors.UnprocessableEntity("Idempotency-Key was already used for a different request"))
		return
	}
	if rec.State != "done" {
		utils.Error(w, apperrors.Conflict("a request with this Idempotency-Key is still in progress"))
		return
	}
	for k, vs := range rec.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set(ReplayedHeader, "true")
	w.WriteHeader(rec.Status)
	_, _ = w.Write(rec.Body)
}

func mutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func hash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// storedHeaders keeps what a replay needs to look like the original, and
// nothing per-request (request IDs, dates, rate-limit counters).
func storedHeaders(h http.Header) map[string][]string {
	out := map[string][]string{}
	for _, k := range []string{"Content-Type", "Location", "Api-Version"} {
		if v := h.Values(k); len(v) > 0 {
			out[k] = v
		}
	}
	return out
}

type recorder struct {
	http.ResponseWriter
	status      int
	body        bytes.Buffer
	overflow    bool
	wroteHeader bool
}

func (r *recorder) WriteHeader(code int) {
	if code >= 100 && code < 200 { // informational (100 Continue): not the final status
		r.ResponseWriter.WriteHeader(code)
		return
	}
	if !r.wroteHeader {
		r.status, r.wroteHeader = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.overflow {
		if r.body.Len()+len(b) > maxStoredBody {
			r.overflow = true
			r.body.Reset()
		} else {
			r.body.Write(b)
		}
	}
	return r.ResponseWriter.Write(b)
}

func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
