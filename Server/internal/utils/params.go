package utils

import (
	"net/http"
	"strconv"
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