package gateway

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/dprince-03/Bibliomania/internal/resilience"
)

// Per-upstream bulkhead size: plenty for normal traffic, but a hung
// upstream can hold at most this many gateway goroutines/connections.
const (
	bulkheadSize = 256
	bulkheadWait = 50 * time.Millisecond
)

var errUpstream5xx = errors.New("upstream returned a gateway/availability error")

// guardedTransport puts one upstream service behind a bulkhead and a
// circuit breaker. Connection errors and 502/503/504 responses count as
// failures; the client still receives the upstream's actual response.
type guardedTransport struct {
	next     http.RoundTripper
	breaker  *resilience.Breaker
	bulkhead *resilience.Bulkhead
}

func newGuardedTransport(name string, next http.RoundTripper) *guardedTransport {
	return &guardedTransport{
		next:     next,
		breaker:  resilience.NewBreaker("http:"+name, func(err error) bool { return err != nil }),
		bulkhead: resilience.NewBulkhead("http:"+name, bulkheadSize, bulkheadWait),
	}
}

func (t *guardedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	release, err := t.bulkhead.Acquire(req.Context())
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	err = t.breaker.Do(func() error {
		var rtErr error
		resp, rtErr = t.next.RoundTrip(req) //nolint:bodyclose // returned to the caller (ReverseProxy), which closes it
		if rtErr != nil {
			return rtErr
		}
		switch resp.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return errUpstream5xx
		}
		return nil
	})
	if errors.Is(err, errUpstream5xx) {
		err = nil
	}
	if err != nil {
		release()
		return nil, err
	}
	// Hold the slot until the body is consumed (streamed downloads count
	// against the bulkhead for as long as they run).
	resp.Body = &releasingBody{ReadCloser: resp.Body, release: release}
	return resp, nil
}

type releasingBody struct {
	io.ReadCloser
	release func()
	done    bool
}

func (b *releasingBody) Close() error {
	err := b.ReadCloser.Close()
	if !b.done {
		b.done = true
		b.release()
	}
	return err
}
