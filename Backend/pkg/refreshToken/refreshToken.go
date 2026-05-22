package refreshtoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// Generate creates a cryptographically secure random refresh token.
// Returns the raw token (sent to client) and its SHA-256 hash (stored in DB).
func Generate() (raw string, hashed string, err error) {
	bytes := make([]byte, 64)
	if _, err = rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	raw = base64.URLEncoding.EncodeToString(bytes)
	hashed = hash(raw)
	return raw, hashed, nil
}

// hash returns the SHA-256 hex hash of a token string.
func hash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// HashToken hashes an incoming token for DB lookup.
func HashToken(raw string) string {
	return hash(raw)
}