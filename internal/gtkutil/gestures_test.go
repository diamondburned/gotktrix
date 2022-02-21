package gtkutil_test

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
)

func ExampleBindRightClick() {
	app := gtk.NewApplication("com.github.diamondburned.gotk4-examples.gtk4.simple", 0)
	app.ConnectActivate(func() {
		l := gtk.NewLabel("Right click or hold me.")

		b := gtk.NewButtonWithLabel("Reset")
		b.SetSensitive(false)
		b.ConnectClicked(func() {
			l.SetLabel("Right click or hold me.")
			b.SetSensitive(false)
		})

		box := gtk.NewBox(gtk.OrientationVertical, 2)
		box.Append(l)
		box.Append(b)

		gtkutil.BindRightClick(box, func() {
			l.SetLabel("Pressed")
			b.SetSensitive(true)
		})

		window := gtk.NewApplicationWindow(app)
		window.SetTitle("gotk4 Example")
		window.SetChild(box)
		window.SetDefaultSize(400, 300)
		window.Show()
	})

	if code := app.Run(nil); code > 0 {
		panic(code)
	}

	// Output:
}
