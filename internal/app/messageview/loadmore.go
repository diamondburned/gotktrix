package messageview

import (
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

type loadMoreButton struct {
	*gtk.Box
	button *gtk.Button

	errRev *gtk.Revealer
	error  *adaptive.ErrorLabel
}

type paginateDoneFunc func(hasMore bool, err error)

var loadMoreCSS = cssutil.Applier("messageview-loadmore", `
	.messageview-loadmore {
		margin: 4px;
		margin-top: 50px;
	}
`)

func newLoadMore(loadMore func(done paginateDoneFunc)) *loadMoreButton {
	b := &loadMoreButton{}
	b.button = gtk.NewButtonWithLabel("More")
	b.button.AddCSSClass("messageview-loadmore-button")
	b.button.SetHAlign(gtk.AlignCenter)
	b.button.SetHasFrame(false)
	b.button.ConnectClicked(func() {
		b.button.SetSensitive(false)
		loadMore(func(hasMore bool, err error) {
			b.done(hasMore)
			if err != nil {
				b.setError(err)
			}
		})
	})

	b.errRev = gtk.NewRevealer()
	b.errRev.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	b.errRev.SetRevealChild(false)

	b.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	b.Box.Append(b.button)
	b.Box.Append(b.errRev)
	loadMoreCSS(b)

	return b
}

func (b *loadMoreButton) setError(err error) {
	b.error = adaptive.NewErrorLabel(err)
	b.error.SetHAlign(gtk.AlignStart)

	b.errRev.SetChild(b.error)
	b.errRev.SetRevealChild(true)
}

func (b *loadMoreButton) done(hasMore bool) {
	b.button.SetSensitive(hasMore)

	b.errRev.SetRevealChild(false)
	b.errRev.SetChild(nil)

	b.error = nil
}
