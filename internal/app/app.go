package app

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
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

// AddActions adds multiple actions and returns a callback that removes all of
// them. Calling the callback is optional.
func (app *Application) AddActions(actions ...gio.Actioner) (rm func()) {
	names := make([]string, len(actions))
	for i, action := range actions {
		app.AddAction(action)
		names[i] = action.Name()
	}

	return func() {
		for _, name := range names {
			app.RemoveAction(name)
		}
	}
}

// AddCallbackAction is a convenient function for adding a SimpleAction.
func (app *Application) AddCallbackAction(name string, f func()) {
	c := gtkutil.NewCallbackAction(name)
	c.OnActivate(f)
	app.AddAction(c)
}

// MenuModel constructs an application-scoped menu model. Components are
// expected to register its preference settings into the menu directly. Most use
// cases of this method should be to add a menu subsection.
func (app *Application) MenuModel() *gio.Menu {
	menu := gio.NewMenu()
	for _, action := range app.ActionGroup.ListActions() {
		menu.Append(action, action)
	}
	return menu
}
