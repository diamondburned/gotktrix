package onlineimage

import (
	"context"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Picture describes an online picture.
type Picture struct {
	*gtk.Picture
	base baseImage
}

// NewPicture creates a new Picture.
func NewPicture(ctx context.Context) *Picture {
	p := Picture{Picture: gtk.NewPicture()}
	p.AddCSSClass("onlineimage")
	p.base.init(ctx, &p)

	return &p
}

// SetMXC sets the Avatar's URL using an MXC URL. It's a convenient function for
// SetURL.
func (p *Picture) SetMXC(mxc matrix.URL) {
	p.SetURL(string(mxc))
}

// SetURL sets the Avatar's URL. URLs are automatically converted if the scheme
// is "mxc".
func (p *Picture) SetURL(url string) {
	p.base.SetFromURL(url)
}

func (p *Picture) setFromPixbuf(pixbuf *gdkpixbuf.Pixbuf) {
	p.SetPixbuf(pixbuf)
}
