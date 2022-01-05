package onlineimage

import (
	"context"
	"net/url"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

type imageParent interface {
	gtk.Widgetter
	setFromPixbuf(p *gdkpixbuf.Pixbuf)
}

type baseImage struct {
	imageParent
	scaler pixbufScaler
	ctx    gtkutil.Cancellable
	url    string
	ok     bool
}

var _ imageParent = (*Avatar)(nil)

// NewAvatar creates a new avatar.
func (b *baseImage) init(ctx context.Context, parent imageParent) {
	b.imageParent = parent
	b.scaler.init(b)

	b.ctx = gtkutil.WithVisibility(ctx, parent)
	b.ctx.OnRenew(func(ctx context.Context) func() {
		b.scaler.Invalidate()
		b.fetch(ctx)
		return nil
	})
}

// SetFromURL sets the Avatar's URL. URLs are automatically converted if the
// scheme is "mxc".
func (b *baseImage) SetFromURL(url string) {
	if b.url == url {
		return
	}

	b.url = url
	b.refetch()
}

func (b *baseImage) refetch() {
	b.ok = false
	b.fetch(b.ctx.Take())
}

func (b *baseImage) size() (w, h int) {
	base := gtk.BaseWidget(b)

	w, h = base.SizeRequest()
	if w > 0 && h > 0 {
		return
	}

	rect := base.Allocation()
	w = rect.Width()
	h = rect.Height()

	return
}

func (b *baseImage) fetch(ctx context.Context) {
	if b.ok || ctx.Err() != nil {
		return
	}

	url := b.url
	if url == "" {
		b.scaler.SetFromPixbuf(nil)
		return
	}

	if urlScheme(url) == "mxc" {
		w, h := b.scaler.ParentSize()
		// Use the maximum scale factor; the scaler will downscale this properly
		// for us.
		url, _ = gotktrix.FromContext(ctx).Thumbnail(matrix.URL(url), w, h, gtkutil.ScaleFactor())
	}

	imgutil.AsyncPixbuf(ctx, url, func(p *gdkpixbuf.Pixbuf) {
		b.ok = true
		b.scaler.SetFromPixbuf(p)
	})
}

func urlScheme(urlStr string) string {
	url, _ := url.Parse(urlStr)
	return url.Scheme
}
