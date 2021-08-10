package app

import (
	"context"
	"log"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

/*
	_diamondburned_ — Today at 16:52
		wow ctx abuse is so fun
		I can't wait until I lose scope of which context has which

	Corporate Shill (SAY NO TO GORM) — Today at 16:58
		This is why you dont do that
		Aaaaaaaaa
		The java compiler does it as well
		Painful
*/

// Application describes the state of a Matrix application.
type Application struct {
	*gtk.Application
	window *gtk.ApplicationWindow
	header *gtk.HeaderBar
}

type ctxKey uint

const (
	applicationKey ctxKey = iota
)

// WithApplication injects the given application instance into a context. The
// returned context will also be cancelled if the application shuts down.
func WithApplication(ctx context.Context, app *Application) context.Context {
	ctx = context.WithValue(ctx, applicationKey, app)

	ctx, cancel := context.WithCancel(ctx)
	app.Connect("shutdown", cancel)

	return ctx
}

// FromContext pulls the application from the given context. If the given
// context isn't derived from Application, then nil is returned.
func FromContext(ctx context.Context) *Application {
	app, _ := ctx.Value(applicationKey).(*Application)
	return app
}

// Wrap wraps a GTK application.
func Wrap(gtkapp *gtk.Application) *Application {
	cssutil.ApplyGlobalCSS()

	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(true)

	spinner := gtk.NewSpinner()
	spinner.Start()
	spinner.SetSizeRequest(18, 18)
	spinner.SetHAlign(gtk.AlignCenter)
	spinner.SetVAlign(gtk.AlignCenter)

	window := gtk.NewApplicationWindow(gtkapp)
	window.SetDefaultSize(600, 400)
	window.SetChild(spinner)
	window.SetTitlebar(header)

	return &Application{
		Application: gtkapp,
		window:      window,
		header:      header,
	}
}

// Error calls Error on the application inside the context. It panics if the
// context does not have the application.
func Error(ctx context.Context, err ...error) {
	FromContext(ctx).Error(err...)
}

// Fatal is similar to Error, but calls Fatal instead.
func Fatal(ctx context.Context, err ...error) {
	FromContext(ctx).Fatal(err...)
}

// Error shows an error popup.
func (app *Application) Error(err ...error) {
	errpopup.Show(&app.window.Window, filterAndLogErrors("error:", err), func() {})
}

// Fatal shows a fatal error popup and closes the application afterwards.
func (app *Application) Fatal(err ...error) {
	errpopup.Fatal(&app.window.Window, filterAndLogErrors("fatal:", err)...)
}

func filterAndLogErrors(prefix string, errors []error) []error {
	nonNils := errors[:0]

	for _, err := range errors {
		if err == nil {
			continue
		}
		nonNils = append(nonNils, err)
		log.Println(prefix, err)
	}

	return nonNils
}

func (app *Application) Window() *gtk.Window    { return &app.window.Window }
func (app *Application) Header() *gtk.HeaderBar { return app.header }

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
