package messageview

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

type extraRevealer struct {
	*gtk.Revealer
	Label *gtk.Label
}

var extraCSS = cssutil.Applier("messageview-extra", `
	.messageview-extra {
		padding: 0 10px;
		margin:  0 10px;
		margin-bottom: -8px; /* I can't believe this works! */
	}
	.messageview-extralabel {
		font-size: .8em;
		border-radius: 5px;
		background-color: mix(@accent_bg_color, @theme_bg_color, 0.75);
	}
`)

func newExtraRevealer() *extraRevealer {
	l := gtk.NewLabel("")
	l.SetXAlign(0)
	l.AddCSSClass("messageview-extralabel")

	r := gtk.NewRevealer()
	r.SetChild(l)
	r.SetRevealChild(false)
	r.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	r.SetTransitionDuration(75)
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
