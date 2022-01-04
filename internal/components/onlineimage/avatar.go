package onlineimage

import (
	"context"
	"net/url"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

// Avatar describes an online avatar.
type Avatar struct {
	*adaptive.Avatar
	ctx gtkutil.Cancellable
	url string
	ok  bool
}

// NewAvatar creates a new avatar.
func NewAvatar(ctx context.Context, size int) *Avatar {
	a := Avatar{Avatar: adaptive.NewAvatar(size)}
	a.AddCSSClass("onlineimage")

	a.ctx = gtkutil.WithVisibility(ctx, a)
	a.ctx.OnRenew(func(ctx context.Context) func() {
		a.fetch(ctx)
		return nil
	})

	return &a
}

// SetFromURL sets the Avatar's URL. URLs are automatically converted if the
// scheme is "mxc".
func (a *Avatar) SetFromURL(url string) {
	a.ok = false
	a.url = url
	a.fetch(a.ctx.Take())
}

// SetFromMXC sets the Avatar's URL using an MXC URL. It's a convenient function
// for SetFromURL.
func (a *Avatar) SetFromMXC(mxc matrix.URL) {
	a.SetFromURL(string(mxc))
}

// Refetch forces the Avatar to refetch the same URL.
func (a *Avatar) Refetch() {
	a.ok = false
	a.fetch(a.ctx.Take())
}

func (a *Avatar) fetch(ctx context.Context) {
	if a.ok || ctx.Err() != nil {
		return
	}

	url := a.url
	if url == "" {
		a.SetFromPaintable(nil)
		return
	}
	if urlScheme(url) == "mxc" {
		size := a.SizeRequest()
		client := gotktrix.FromContext(ctx)
		url, _ = client.SquareThumbnail(matrix.URL(url), size, gtkutil.ScaleFactor())
	}

	imgutil.AsyncGET(ctx, url, func(p gdk.Paintabler) {
		a.ok = true
		a.Avatar.SetFromPaintable(p)
	})
}

func urlScheme(urlStr string) string {
	url, _ := url.Parse(urlStr)
	return url.Scheme
}
