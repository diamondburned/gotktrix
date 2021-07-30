package app

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// Application describes the state of a Matrix application.
type Application struct {
	*gtk.Application
	Window *gtk.ApplicationWindow
	Header *gtk.HeaderBar
	Client *gotktrix.Client
}

// Wrap wraps a GTK application.
func Wrap(app *gtk.Application) *Application {
	cssutil.ApplyGlobalCSS()

	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(true)

	spinner := gtk.NewSpinner()
	spinner.Start()
	spinner.SetSizeRequest(18, 18)
	spinner.SetHAlign(gtk.AlignCenter)
	spinner.SetVAlign(gtk.AlignCenter)

	window := gtk.NewApplicationWindow(app)
	window.SetDefaultSize(600, 400)
	window.SetChild(spinner)
	window.SetTitlebar(header)

	return &Application{
		Application: app,
		Window:      window,
		Header:      header,
	}
}

// Error shows an error popup.
func (app *Application) Error(err ...error) {
	for _, err := range err {
		log.Println("error:", err)
	}

	errpopup.Show(&app.Window.Window, err, func() {})
}

// Fatal shows a fatal error popup and closes the application afterwards.
func (app *Application) Fatal(err ...error) {
	for _, err := range err {
		log.Println("fatal:", err)
	}

	errpopup.Fatal(&app.Window.Window, err...)
}
