package messageview

import (
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

type extraRevealer struct {
	*gtk.Revealer
	Label *gtk.Label
}

var extraCSS = cssutil.Applier("messageview-extra", `
	@keyframes messageview-extra-breathing {
		  0% { opacity: 1.0; }
		 75% { opacity: 1.0; }
		100% { opacity: 0.7; }
	}

	.messageview-extra {
		padding: 0;
		margin:  0 10px;
		margin-bottom: -8px; /* I can't believe this works! */
	}
	.messageview-extra > * {
		padding: 0 5px;
		border-radius: 5px;
		background-color: mix(@accent_bg_color, @theme_bg_color, 0.75);
	}
	.messageview-extralabel {
		padding: 0;
		font-size: .8em;
		animation: breathing 800ms infinite alternate;
	}
`)

func newExtraRevealer() *extraRevealer {
	l := gtk.NewLabel("")
	l.SetXAlign(0)
	l.AddCSSClass("messageview-extralabel")

	b := adw.NewBin()
	b.SetChild(l)

	r := gtk.NewRevealer()
	r.SetChild(b)
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
