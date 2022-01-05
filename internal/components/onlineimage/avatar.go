package onlineimage

import (
	"context"
	"net/url"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

// Avatar describes an online avatar.
type Avatar struct {
	*adaptive.Avatar
	scaler pixbufScaler
	ctx    gtkutil.Cancellable
	url    string
	ok     bool
}

var _ imageParent = (*Avatar)(nil)

// NewAvatar creates a new avatar.
func NewAvatar(ctx context.Context, size int) *Avatar {
	a := Avatar{Avatar: adaptive.NewAvatar(size)}
	a.AddCSSClass("onlineimage")

	a.scaler.init(&a)

	a.ctx = gtkutil.WithVisibility(ctx, &a)
	a.ctx.OnRenew(func(ctx context.Context) func() {
		a.scaler.Invalidate()
		a.fetch(ctx)
		return nil
	})

	return &a
}

// SetFromMXC sets the Avatar's URL using an MXC URL. It's a convenient function
// for SetFromURL.
func (a *Avatar) SetFromMXC(mxc matrix.URL) {
	a.SetFromURL(string(mxc))
}

// SetFromURL sets the Avatar's URL. URLs are automatically converted if the
// scheme is "mxc".
func (a *Avatar) SetFromURL(url string) {
	if a.url == url {
		return
	}

	a.url = url
	a.refetch()
}

func (a *Avatar) refetch() {
	a.ok = false
	a.fetch(a.ctx.Take())
}

func (a *Avatar) fetch(ctx context.Context) {
	if a.ok || ctx.Err() != nil {
		return
	}

	url := a.url
	if url == "" {
		a.scaler.SetFromPixbuf(nil)
		return
	}

	if urlScheme(url) == "mxc" {
		w, h := a.scaler.ParentSize()
		// Use the maximum scale factor; the scaler will downscale this properly
		// for us.
		url, _ = gotktrix.FromContext(ctx).Thumbnail(matrix.URL(url), w, h, gtkutil.ScaleFactor())
	}

	imgutil.AsyncPixbuf(ctx, url, func(p *gdkpixbuf.Pixbuf) {
		a.ok = true
		a.scaler.SetFromPixbuf(p)
	})
}

func urlScheme(urlStr string) string {
	url, _ := url.Parse(urlStr)
	return url.Scheme
}
