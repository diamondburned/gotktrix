package gtkutil

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// BindRightClick binds the given widget to take in right-click gestures. The
// function will also check for long-hold gestures.
func BindRightClick(w gtk.Widgetter, f func()) {
	c := gtk.NewGestureClick()
	c.SetButton(3)       // secondary
	c.SetExclusive(true) // handle mouse only
	c.Connect("pressed", f)

	l := gtk.NewGestureLongPress()
	l.Connect("pressed", f)

	w.AddController(c)
	w.AddController(l)
}
