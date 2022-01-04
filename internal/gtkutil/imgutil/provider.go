package imgutil

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
)

// Providers holds multiple providers.
type Providers map[string]Provider

// NewProviders creates a new Providers instance.
func NewProviders(providers ...Provider) Providers {
	m := make(Providers, len(providers))
	for _, prov := range providers {
		for _, scheme := range prov.Schemes() {
			m[scheme] = prov
		}
	}
	return m
}

// AsyncDo invokes any of the providers inside.
func (p Providers) AsyncDo(ctx context.Context, uri string, f func(gdk.Paintabler), opts ...Opts) {
	url, err := url.Parse(uri)
	if err != nil {
		OptsError(opts, err)
		return
	}

	provider, ok := p[url.Scheme]
	if !ok {
		OptsError(opts, fmt.Errorf("unknown scheme %q", url.Scheme))
	}

	provider.AsyncDo(ctx, uri, f, opts...)
}

// Provider describes a universal resource provider.
type Provider interface {
	Schemes() []string
	AsyncDo(ctx context.Context, url string, f func(gdk.Paintabler), opts ...Opts)
}

type httpProvider []string

// HTTPProvider is the universal resource provider that handles HTTP and HTTPS
// schemes.
var HTTPProvider Provider = httpProvider{"http", "https"}

// Schemes implements Provider.
func (p httpProvider) Schemes() []string {
	return []string(p)
}

// AsyncDo implements Provider.
func (p httpProvider) AsyncDo(ctx context.Context, url string, f func(gdk.Paintabler), opts ...Opts) {
	AsyncGET(ctx, url, f, opts...)
}

type mxcProvider struct {
	Width  int
	Height int
	Flags  MatrixImageFlags
}

// MatrixImageFlags is describes boolean attributes for fetching Matrix images.
type MatrixImageFlags uint8

const (
	_ MatrixImageFlags = iota
	// MatrixNoCrop asks the server to scale the image down to fit the frame
	// instead of cropping the image.
	MatrixNoCrop
)

// Has returns true if f has this.
func (f MatrixImageFlags) Has(this MatrixImageFlags) bool {
	return f&this == this
}

// MXCProvider returns a new universal resource provider that handles MXC URLs.
func MXCProvider(w, h int, flags MatrixImageFlags) Provider {
	return mxcProvider{w, h, flags}
}

// Schemes implements Provider.
func (p mxcProvider) Schemes() []string {
	return []string{"mxc"}
}

// AsyncDo implements Provider.
func (p mxcProvider) AsyncDo(ctx context.Context, mxc string, f func(gdk.Paintabler), opts ...Opts) {
	client := gotktrix.FromContext(ctx)
	if client == nil {
		OptsError(opts, errors.New("context missing gotktrix.Client"))
		return
	}

	var url string
	if p.Flags.Has(MatrixNoCrop) {
		url, _ = client.ScaledThumbnail(matrix.URL(mxc), p.Width, p.Height, gtkutil.ScaleFactor())
	} else {
		url, _ = client.Thumbnail(matrix.URL(mxc), p.Width, p.Height, gtkutil.ScaleFactor())
	}

	if url == "" {
		return
	}

	AsyncGET(ctx, url, f, opts...)
}
