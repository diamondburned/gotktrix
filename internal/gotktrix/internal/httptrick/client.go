package httptrick

import (
	"log"
	"net/http"
	"sync"

	"github.com/diamondburned/gotktrix/internal/registry"
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

// Interceptor wraps around an existing roundtripper and calls the registered
// callbacks on each request.
type Interceptor struct {
	r http.RoundTripper
	u sync.RWMutex
	m registry.Registry
}

// InterceptFunc is a function for intercepting a request. The request being
// intercepted is given, as well as a callback that triggers the actual request.
type InterceptFunc func(*http.Request, func() error) error

// InterceptFullFunc is the full version of InterceptFunc: the next callback
// also returns a response.
type InterceptFullFunc func(*http.Request, func() (*http.Response, error)) (*http.Response, error)

// WrapInterceptor wraps the given RoundTripper inside a Interceptor.
func WrapInterceptor(c http.RoundTripper) *Interceptor {
	if c == nil {
		c = http.DefaultTransport
	}

	return &Interceptor{
		r: c,
		m: registry.New(2),
	}
}

func (r *Interceptor) RoundTrip(req *http.Request) (*http.Response, error) {
	do := func() (*http.Response, error) {
		r, err := r.r.RoundTrip(req)
		if err == nil && r == nil {
			log.Println("base RoundTripper impl returned nil (r, err)")
		}
		return r, err
	}

	r.u.RLock()
	r.m.Each(func(v, _ interface{}) {
		f := v.(InterceptFullFunc)
		next := do
		do = func() (*http.Response, error) {
			return f(req, next)
		}
	})
	r.u.RUnlock()

	return do()
}

// AddIntercept adds the given callback. The callback is called when RoundTrip
// is called.
func (r *Interceptor) AddIntercept(f InterceptFunc) func() {
	return r.AddInterceptFull(
		func(r *http.Request, next func() (*http.Response, error)) (*http.Response, error) {
			var resp *http.Response
			do := func() (err error) {
				resp, err = next()
				return
			}

			err := f(r, do)
			return resp, err
		},
	)
}

// AddIntercept adds the given callback. The callback is called when RoundTrip
// is called.
func (r *Interceptor) AddInterceptFull(f InterceptFullFunc) func() {
	r.u.Lock()
	v := r.m.Add(f, nil)
	r.u.Unlock()

	return func() {
		r.u.Lock()
		v.Delete()
		r.u.Unlock()
	}
}
