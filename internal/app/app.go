package app

import (
	"context"
	"log"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
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

func init() {
	glib.LogUseDefaultLogger()
}

// SuffixedTitle suffixes the title with the gotktrix label.
func SuffixedTitle(title string) string {
	if title == "" {
		return "gotktrix"
	}
	return title + " — gotktrix"
}

// Application describes the state of a Matrix application.
type Application struct {
	*gtk.Application
	window *gtk.ApplicationWindow
	header gtk.Widgetter
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

// SetTitle sets the main window's title.
func SetTitle(ctx context.Context, title string) {
	FromContext(ctx).SetTitle(title)
}

// Window returns the context's window.
func Window(ctx context.Context) *gtk.Window {
	return FromContext(ctx).Window()
}

// FromContext pulls the application from the given context. If the given
// context isn't derived from Application, then nil is returned.
func FromContext(ctx context.Context) *Application {
	app, _ := ctx.Value(applicationKey).(*Application)
	return app
}

// OpenURI opens the given URI using the system's default application.
func OpenURI(ctx context.Context, uri string) {
	if uri == "" {
		return
	}
	ts := uint32(time.Now().Unix())
	gtk.ShowURI(FromContext(ctx).Window(), uri, ts)
}

// Wrap wraps a GTK application.
func Wrap(gtkapp *gtk.Application) *Application {
	cssutil.ApplyGlobalCSS()

	window := gtk.NewApplicationWindow(gtkapp)
	window.SetDefaultSize(600, 400)

	// Initialize the scale factor state.
	gtkutil.ScaleFactor()

	app := &Application{
		Application: gtkapp,
		window:      window,
	}
	app.SetLoading()

	return app
}

// Error calls Error on the application inside the context. It panics if the
// context does not have the application.
func Error(ctx context.Context, errs ...error) {
	for _, err := range errs {
		log.Println("error:", err)
	}

	if app := FromContext(ctx); app != nil {
		app.Error(errs...)
	}
}

// Fatal is similar to Error, but calls Fatal instead.
func Fatal(ctx context.Context, errs ...error) {
	for _, err := range errs {
		log.Println("fatal:", err)
	}

	if app := FromContext(ctx); app != nil {
		app.Fatal(errs...)
	} else {
		panic("fatal error(s) occured")
	}
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

// SetLoading shows a spinning circle. It disables the window.
func (app *Application) SetLoading() {
	spinner := gtk.NewSpinner()
	spinner.SetSizeRequest(24, 24)
	spinner.SetHAlign(gtk.AlignCenter)
	spinner.SetVAlign(gtk.AlignCenter)
	spinner.Start()

	app.window.SetChild(spinner)
	app.SetTitle("Loading")
	app.NotifyChild(true, func() { spinner.Stop() })
}

// NotifyChild calls f if the main window's child is changed. If once is true,
// then f is never called again.
func (app *Application) NotifyChild(once bool, f func()) {
	var childHandle glib.SignalHandle
	childHandle = app.window.Connect("notify::child", func() {
		f()
		app.window.HandlerDisconnect(childHandle)
	})
}

// SetSensitive sets whether or not the application's window is enabled.
func (app *Application) SetSensitive(sensitive bool) {
	app.window.SetSensitive(sensitive)
}

// Window returns the main instance's window.
func (app *Application) Window() *gtk.Window {
	return &app.window.Window
}

// NewHeader creates a new header and puts it into the application window.
func (app *Application) NewHeader() *gtk.HeaderBar {
	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(true)

	app.header = header
	app.window.SetTitlebar(app.header)

	return header
}

// NewWindowHeader creates a new blank header.
func (app *Application) NewWindowHandle() *gtk.WindowHandle {
	header := gtk.NewWindowHandle()

	app.header = header
	app.window.SetTitlebar(app.header)

	return header
}

// SetTitle sets the application (and the main instance window)'s title.
func (app *Application) SetTitle(title string) {
	app.window.SetTitle(SuffixedTitle(title))
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
