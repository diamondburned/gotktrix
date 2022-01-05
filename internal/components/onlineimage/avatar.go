package onlineimage

import (
	"context"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
)

// Avatar describes an online avatar.
type Avatar struct {
	*adaptive.Avatar
	base baseImage
}

var _ imageParent = (*Avatar)(nil)

// NewAvatar creates a new avatar.
func NewAvatar(ctx context.Context, size int) *Avatar {
	a := Avatar{Avatar: adaptive.NewAvatar(size)}
	a.AddCSSClass("onlineimage")
	a.base.init(ctx, &a)

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
	a.base.SetFromURL(url)
}

func (a *Avatar) setFromPixbuf(p *gdkpixbuf.Pixbuf) {
	a.SetFromPixbuf(p)
}
