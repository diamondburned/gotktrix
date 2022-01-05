package onlineimage

import (
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
)

type imageParent interface {
	gtk.Widgetter
	SetFromPixbuf(p *gdkpixbuf.Pixbuf)
	refetch()
}

// maxScale is the maximum supported scale that we can supply a properly scaled
// pixbuf for.
const maxScale = 3

// 1x, 2x and 3x
type pixbufScales [maxScale]*gdkpixbuf.Pixbuf

type pixbufScaler struct {
	parent imageParent
	// parentSz keeps track of the parent widget's sizes in case it has been
	// changed, which would force us to invalidate all scaled pixbufs.
	parentSz [2]int
	// scales stores scaled pixbufs.
	scales pixbufScales
	// src is the source pixbuf.
	src *gdkpixbuf.Pixbuf
	// maxed is true if the source pixbuf cannot go any higher.
	maxed bool
}

// SetFromPixbuf invalidates and sets the internal scaler's pixbuf. The
// SetFromPixbuf call might be bubbled up to the parent widget.
func (p *pixbufScaler) SetFromPixbuf(pixbuf *gdkpixbuf.Pixbuf) {
	p.src = pixbuf
	p.maxed = false
	p.scales = pixbufScales{}

	p.invalidate(true)
}

// Invalidate prompts the scaler to rescale.
func (p *pixbufScaler) Invalidate() {
	p.invalidate(false)
}

// ParentSize gets the cached parent widget's size request.
func (p *pixbufScaler) ParentSize() (w, h int) {
	return p.parentSz[0], p.parentSz[1]
}

func (p *pixbufScaler) init(parent imageParent) {
	base := gtk.BaseWidget(parent)
	w, h := base.SizeRequest()

	p.parent = parent
	p.parentSz = [2]int{w, h}

	base.Connect("notify::scale-factor", func() {
		gtkutil.SetScaleFactor(gtk.BaseWidget(p.parent).ScaleFactor())
		p.Invalidate()
	})
}

// invalidate invalidates the scaled pixbuf and optionally refetches one if
// needed. The user should use this method instead of calling on the parent
// widget's Refetch method.
func (p *pixbufScaler) invalidate(newPixbuf bool) {
	if p.src == nil {
		p.parent.SetFromPixbuf(nil)
		return
	}

	parent := gtk.BaseWidget(p.parent)

	scale := parent.ScaleFactor()
	if scale == 0 {
		// No allocations yet.
		return
	}

	dstW, dstH := parent.SizeRequest()
	if p.parentSz != [2]int{dstW, dstH} {
		// Size changed, so invalidate all known pixbufs.
		p.scales = pixbufScales{}
		p.parentSz = [2]int{dstW, dstH}
	}

	// Scale the width and height up.
	dstW *= scale
	dstH *= scale

	srcW := p.src.Width()
	srcH := p.src.Height()

	if !p.maxed && (dstW > srcW || dstH > srcH) {
		if newPixbuf {
			p.maxed = true
			p.parent.SetFromPixbuf(p.src)
		} else {
			p.parent.refetch()
		}
		return
	}

	if scale > maxScale {
		// We don't have these scales, so just use the source. User gets jagged
		// image, but on a 3x HiDPI display, it doesn't matter, unless the user
		// has both 3x and 1x displays.
		p.parent.SetFromPixbuf(p.src)
		return
	}

	pixbuf := p.scales[scale-1]
	if pixbuf == nil {
		// InterpTiles is apparently as good as bilinear when downscaling, which
		// is what we want.
		pixbuf = p.src.ScaleSimple(dstW, dstH, gdkpixbuf.InterpTiles)
		p.scales[scale-1] = pixbuf
	}

	p.parent.SetFromPixbuf(pixbuf)
}
