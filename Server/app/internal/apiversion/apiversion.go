// Package apiversion implements the gateway's REST API versioning policy.
//
// Strategy: URI-path versioning — /api/v1/..., /api/v2/... — chosen over
// header or media-type negotiation because every client here (Next.js
// server code, Flutter, JavaFX, nginx routing, the Swagger UI) can see and
// route on a path trivially, and a versioned URL is cacheable and
// bookmarkable as-is.
//
// Rules (also in Server/app/docs/API.md):
//   - A version is only bumped for a breaking change (removing/renaming a
//     field or route, changing a type or a status code's meaning).
//     Additive changes (new optional field, new route) ship in place.
//   - A new version runs alongside the old one; the old one is announced
//     deprecated (Deprecation + Sunset + Link headers on every response)
//     and removed only after its sunset date.
//   - A request for a version the gateway doesn't serve gets a JSON 404
//     listing the supported versions, never a silent fallback.
//
// gRPC contracts are versioned by proto package (bibliomania.<svc>.v1),
// events by an explicit schema_version per event type
// (internal/events), and the GraphQL schema evolves additively with
//
//	@deprecated	— see those packages.
package apiversion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Version describes one served REST API version.
type Version struct {
	Name         string     `json:"version"`
	Status       string     `json:"status"` // "current" | "supported" | "deprecated"
	BasePath     string     `json:"base_path"`
	DeprecatedOn *time.Time `json:"deprecated_on,omitempty"`
	SunsetOn     *time.Time `json:"sunset_on,omitempty"`
}

type Registry struct {
	versions map[string]*Version
	order    []string
	current  string
}

var versionSegment = regexp.MustCompile(`^v[0-9]+$`)

// NewRegistry builds the registry from the versions the gateway serves
// (oldest first; the last is "current") and API_DEPRECATED_VERSIONS.
func NewRegistry(served []string, deprecations string) (*Registry, error) {
	if len(served) == 0 {
		return nil, fmt.Errorf("apiversion: no versions served")
	}
	r := &Registry{versions: map[string]*Version{}, order: served, current: served[len(served)-1]}
	for _, v := range served {
		status := "supported"
		if v == r.current {
			status = "current"
		}
		r.versions[v] = &Version{Name: v, Status: status, BasePath: "/api/" + v}
	}

	for _, entry := range strings.Split(deprecations, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		v, ok := r.versions[parts[0]]
		if !ok || len(parts) < 2 {
			return nil, fmt.Errorf("apiversion: bad API_DEPRECATED_VERSIONS entry %q", entry)
		}
		on, err := time.Parse(time.DateOnly, parts[1])
		if err != nil {
			return nil, fmt.Errorf("apiversion: bad deprecation date in %q: %w", entry, err)
		}
		v.Status, v.DeprecatedOn = "deprecated", &on
		if len(parts) > 2 && parts[2] != "" {
			sunset, err := time.Parse(time.DateOnly, parts[2])
			if err != nil {
				return nil, fmt.Errorf("apiversion: bad sunset date in %q: %w", entry, err)
			}
			v.SunsetOn = &sunset
		}
	}
	return r, nil
}

// Supported lists served version names, oldest first.
func (r *Registry) Supported() []string {
	return slices.Clone(r.order)
}

// Middleware validates the version segment of every /api/v*/ request,
// rejects unknown versions, and stamps version/deprecation headers.
// Non-/api paths (/graphql, /health, /swagger, ...) pass through.
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rest, ok := strings.CutPrefix(req.URL.Path, "/api/")
		if !ok || rest == "versions" {
			next.ServeHTTP(w, req)
			return
		}

		segment, _, _ := strings.Cut(rest, "/")
		v, known := r.versions[segment]
		if !versionSegment.MatchString(segment) || !known {
			r.writeUnsupported(w, segment)
			return
		}

		w.Header().Set("API-Version", v.Name)
		if v.DeprecatedOn != nil {
			// RFC 9745 (Deprecation), RFC 8594 (Sunset).
			w.Header().Set("Deprecation", fmt.Sprintf("@%d", v.DeprecatedOn.Unix()))
			if v.SunsetOn != nil {
				w.Header().Set("Sunset", v.SunsetOn.UTC().Format(http.TimeFormat))
			}
			w.Header().Add("Link", fmt.Sprintf(`</api/%s>; rel="successor-version"`, r.current))
			w.Header().Add("Link", `</api/versions>; rel="deprecation"; type="application/json"`)
		}
		next.ServeHTTP(w, req)
	})
}

type unsupported struct {
	Success           bool     `json:"success"`
	Error             string   `json:"error"`
	Code              int      `json:"code"`
	SupportedVersions []string `json:"supported_versions"`
}

func (r *Registry) writeUnsupported(w http.ResponseWriter, segment string) {
	msg := "API version missing from path — use /api/{version}/..."
	if segment != "" {
		msg = fmt.Sprintf("unsupported API version %q", segment)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(unsupported{
		Success: false, Error: msg, Code: http.StatusNotFound, SupportedVersions: r.Supported(),
	})
}

type versionsResponse struct {
	Current  string     `json:"current"`
	Versions []*Version `json:"versions"`
}

// Handle serves GET /api/versions: every REST API version this server
// serves, its status, and for deprecated ones the deprecation and sunset
// dates. Deliberately unversioned itself (it sits outside /api/v1, so it's
// documented in Server/app/docs/API.md rather than the v1 Swagger spec).
func (r *Registry) Handle(w http.ResponseWriter, _ *http.Request) {
	resp := versionsResponse{Current: r.current}
	for _, name := range r.order {
		resp.Versions = append(resp.Versions, r.versions[name])
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "api versions", "data": resp})
}
