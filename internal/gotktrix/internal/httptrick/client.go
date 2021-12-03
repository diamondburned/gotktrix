package httptrick

import (
	"net/http"
	"sync"

	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/registry"
)

// RoundTripWrapper wraps a http.RoundTripper for debugging.
type RoundTripWrapper struct {
	Old http.RoundTripper
	F   func(*http.Request, *http.Response)
}

func (c RoundTripWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.Old == nil {
		c.Old = http.DefaultTransport
	}

	r, err := c.Old.RoundTrip(req)
	if err != nil {
		return r, err
	}

	c.F(req, r)
	return r, nil
}

// RoundTripWarner wraps around an existing roundtripper and calls certain
// callbacks on every error.
type RoundTripWarner struct {
	r http.RoundTripper
	u sync.Mutex
	m registry.M
}

// WrapRoundTripWarner wraps the given RoundTripper inside a RoundTripWarner.
func WrapRoundTripWarner(c http.RoundTripper) *RoundTripWarner {
	if c == nil {
		c = http.DefaultTransport
	}

	return &RoundTripWarner{
		r: c,
		m: make(registry.M),
	}
}

func (r *RoundTripWarner) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.r.RoundTrip(req)
	if err == nil {
		return resp, nil
	}

	r.u.Lock()
	r.m.Each(func(f interface{}) {
		f.(func(*http.Request, error))(req, err)
	})
	r.u.Unlock()

	return resp, err
}

// OnError adds the given callback into the warner. The callback is called when
// RoundTrip errors out.
func (r *RoundTripWarner) OnError(f func(*http.Request, error)) func() {
	r.u.Lock()
	v := r.m.Add(f)
	r.u.Unlock()

	return func() {
		r.u.Lock()
		v.Delete()
		r.u.Unlock()
	}
}
