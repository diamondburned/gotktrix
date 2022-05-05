package messageview

import (
	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/animations"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

type extraRevealer struct {
	*gtk.Revealer
	Label *gtk.Label
}

//go:embed styles/messageview-extra.css
var extraStyle string
var extraCSS = cssutil.Applier("messageview-extra", extraStyle)

func newExtraRevealer() *extraRevealer {
	l := gtk.NewLabel("")
	l.SetXAlign(0)
	l.AddCSSClass("messageview-extralabel")

	b := gtk.NewBox(gtk.OrientationHorizontal, 0)
	b.Append(animations.NewBreathingDots())
	b.Append(l)

	r := gtk.NewRevealer()
	r.SetChild(b)
	r.SetCanTarget(false)
	r.SetRevealChild(false)
	r.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	r.SetTransitionDuration(150)
	extraCSS(r)

	return &extraRevealer{
		Revealer: r,
		Label:    l,
	}
}

func (r *extraRevealer) Clear() {
	r.Label.SetLabel("")
	r.Revealer.SetRevealChild(false)
}

func (r *extraRevealer) SetMarkup(markup string) {
	if markup == "" {
		r.Clear()
		return
	}

	r.Revealer.SetRevealChild(true)
	r.Label.SetMarkup(markup)
}
