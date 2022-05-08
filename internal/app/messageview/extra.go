package messageview

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/animations"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

type extraRevealer struct {
	*gtk.Revealer
	Label *gtk.Label
}

var extraCSS = cssutil.Applier("messageview-extra", `
	.messageview-extra {
		padding: 0;
		margin:  0 10px;
		margin-bottom: -10px; /* I can't believe this works! */
	}
	.messageview-extra > * {
		padding: 0 5px;
		border-radius: 5px;
		background-color: mix(@theme_selected_bg_color, @theme_bg_color, 0.75);
	}
	.messageview-extralabel {
		padding-left: 4px;
		font-size: .8em;
	}
`)

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
