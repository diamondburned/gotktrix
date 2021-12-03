package httptrick

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TransportHeaderOverride overrides the header of all incoming responses
// matching the path inside the map. It is used for caching.
type TransportHeaderOverride struct {
	R http.RoundTripper
	H map[string]map[string]string
}

func (t TransportHeaderOverride) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.R == nil {
		t.R = http.DefaultTransport
	}

	resp, err := t.R.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if h, ok := t.H[req.URL.Path]; ok {
		overrideHeader(resp, h)
	} else {
		// Slow path: globbing.
		for p, h := range t.H {
			if !strings.HasSuffix(p, "*") {
				continue
			}

			p = strings.TrimSuffix(p, "*")

			if strings.HasPrefix(req.URL.Path, p) {
				overrideHeader(resp, h)
				break
			}
		}
	}

	return resp, nil
}

func overrideHeader(resp *http.Response, h map[string]string) {
	for k, v := range h {
		resp.Header.Set(k, v)
	}
}

// OverrideCacheControl creates a value that overrides the Cache-Control header.
func OverrideCacheControl(age time.Duration) string {
	return fmt.Sprintf(
		// https://cache-control.sdgluck.vercel.app/
		"public, max-age %.0f, max-stale %.0f",
		(age * 1).Seconds(),
		(age * 2).Seconds(),
	)
}
