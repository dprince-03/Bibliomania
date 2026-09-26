package utils

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// GetPathID extracts a uint64 ID from a URL path parameter.
// Uses Go 1.22's PathValue method.
// Returns 0 if the parameter is missing or invalid.
func GetPathID(r *http.Request, param string) uint64 {
	val := r.PathValue(param)
	if val == "" {
		return 0
	}
	id, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// Uint64Key formats an ID as an event/partition key.
func Uint64Key(id uint64) string {
	return strconv.FormatUint(id, 10)
}

// ResourceRef is a path parameter that may be either an internal numeric
// ID or a public UUID (see Server/app/docs/API.md → "IDs").
type ResourceRef struct {
	ID       uint64
	PublicID string // canonical lower-case UUID, when that's what was given
}

// GetPathRef parses param as a numeric ID or a UUID. ok is false if it's
// neither.
func GetPathRef(r *http.Request, param string) (ref ResourceRef, ok bool) {
	val := r.PathValue(param)
	if id, err := strconv.ParseUint(val, 10, 64); err == nil && id > 0 {
		return ResourceRef{ID: id}, true
	}
	if u, err := uuid.Parse(val); err == nil {
		return ResourceRef{PublicID: u.String()}, true
	}
	return ResourceRef{}, false
}

// NewPublicID returns a UUIDv7 (time-ordered) for a new record.
func NewPublicID() string {
	return uuid.Must(uuid.NewV7()).String()
}
