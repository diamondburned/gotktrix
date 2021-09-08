package messageview

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

type extraRevealer struct {
	*gtk.Revealer
	Label *gtk.Label
}

func newExtraRevealer() *extraRevealer {
	l := gtk.NewLabel("")
	l.AddCSSClass("messageview-extralabel")

	r := gtk.NewRevealer()
	r.SetChild(l)
	r.SetRevealChild(false)
	r.SetTransitionType(gtk.RevealerTransitionTypeSlideUp)
	r.AddCSSClass("messageview-extra")

	return &extraRevealer{
		Revealer: r,
		Label:    l,
	}
}
