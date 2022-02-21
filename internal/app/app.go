package app

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
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
	ctx context.Context // non-nil if Run
}

type ctxKey uint

const (
	applicationKey ctxKey = iota
	windowKey      ctxKey = iota
)

// WithApplication injects the given application instance into a context. The
// returned context will also be cancelled if the application shuts down.
func WithApplication(ctx context.Context, app *Application) context.Context {
	ctx = context.WithValue(ctx, applicationKey, app)

	ctx, cancel := context.WithCancel(ctx)
	app.ConnectShutdown(cancel)

	return ctx
}

// FromContext pulls the application from the given context. If the given
// context isn't derived from Application, then nil is returned.
func FromContext(ctx context.Context) *Application {
	app, _ := ctx.Value(applicationKey).(*Application)
	return app
}

// IsActive returns true if any of the windows belonging to gotktrix is active.
func IsActive(ctx context.Context) bool {
	app := FromContext(ctx)
	for _, win := range app.Windows() {
		if win.IsActive() {
			return true
		}
	}
	return false
}

// New creates a new Application.
func New(id string) *Application {
	return &Application{
		Application: gtk.NewApplication(id, gio.ApplicationFlagsNone),
	}
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

	if win := WindowFromContext(ctx); win != nil {
		win.Fatal(errs...)
		return
	}

	if app := FromContext(ctx); app != nil {
		app.Fatal(errs...)
		return
	}

	panic("fatal error(s) occured")
}

// Error shows an error popup.
func (app *Application) Error(err ...error) {
	errpopup.Show(app.ActiveWindow(), filterAndLogErrors("error:", err), func() {})
}

// Fatal shows a fatal error popup and closes the application afterwards.
func (app *Application) Fatal(err ...error) {
	for _, win := range app.Windows() {
		win := win
		win.SetSensitive(false)
		errpopup.Show(&win, filterAndLogErrors("fatal:", err), app.Quit)
	}
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

// ConnectActivate connects f to be called when Application is activated.
func (app *Application) ConnectActivate(f func(ctx context.Context)) {
	app.Application.ConnectActivate(func() {
		if app.ctx == nil {
			panic("unreachable")
		}
		f(app.ctx)
	})
}

// Quit quits the application. The function is thread-safe.
func (app *Application) Quit() {
	glib.IdleAddPriority(coreglib.PriorityHigh, app.Application.Quit)
}

// Run runs the application for as long as the context is alive.
func (app *Application) Run(ctx context.Context, args []string) int {
	if app.ctx != nil {
		panic("Run called more than once")
	}
	if ctx == nil {
		panic("Run given a nil context")
	}

	app.ctx = WithApplication(ctx, app)

	go func() {
		<-ctx.Done()
		app.Quit()
	}()

	return app.Application.Run(args)
}

// NewWindow creates a new Window.
func (app *Application) NewWindow() *Window {
	cssutil.ApplyGlobalCSS()

	window := gtk.NewApplicationWindow(app.Application)
	window.SetDefaultSize(600, 400)

	// Initialize the scale factor state.
	gtkutil.ScaleFactor()

	w := Window{Window: window.Window}
	w.SetLoading()

	return &w
}

// AddActions adds the given map of actions into the Application.
func (app *Application) AddActions(m map[string]func()) {
	for name, fn := range m {
		name = strings.TrimPrefix(name, "app.")

		c := gtkutil.NewCallbackAction(name)
		c.OnActivate(fn)
		app.AddAction(c)
	}
}

// AddActionCallbacks is the ActionCallback variant of AddActions.
func (app *Application) AddActionCallbacks(m map[string]gtkutil.ActionCallback) {
	for name, callback := range m {
		name = strings.TrimPrefix(name, "app.")

		action := gio.NewSimpleAction(name, callback.ArgType)
		action.ConnectActivate(callback.Func)
		app.AddAction(action)
	}
}

// SendNotification sends the given notification asynchronously.
func (app *Application) SendNotification(n Notification) {
	n.send(&app.Application.Application)
}

// Window wraps a gtk.ApplicationWindow.
type Window struct {
	gtk.Window
}

// WithWindow injects the given Window instance into a context. The returned
// context will be cancelled if the window is closed.
func WithWindow(ctx context.Context, win *Window) context.Context {
	ctx = context.WithValue(ctx, windowKey, win)

	ctx, cancel := context.WithCancel(ctx)
	win.ConnectDestroy(cancel)

	return ctx
}

// WindowFromContext returns the context's window.
func WindowFromContext(ctx context.Context) *Window {
	win, _ := ctx.Value(windowKey).(*Window)
	return win
}

// GTKWindowFromContext returns the context's window. If the context does not
// have a window, then the active window is returned.
func GTKWindowFromContext(ctx context.Context) *gtk.Window {
	win, _ := ctx.Value(windowKey).(*Window)
	if win != nil {
		return &win.Window
	}

	app := FromContext(ctx)
	return app.ActiveWindow()
}

// SetTitle sets the main window's title.
func SetTitle(ctx context.Context, title string) {
	WindowFromContext(ctx).SetTitle(title)
}

// OpenURI opens the given URI using the system's default application.
func OpenURI(ctx context.Context, uri string) {
	if uri == "" {
		return
	}
	ts := uint32(time.Now().Unix())
	gtk.ShowURI(GTKWindowFromContext(ctx), uri, ts)
}

// Error shows an error popup.
func (w *Window) Error(err ...error) {
	errpopup.Show(&w.Window, filterAndLogErrors("error:", err), func() {})
}

// Fatal shows a fatal error popup and closes the window afterwards.
func (w *Window) Fatal(err ...error) {
	errpopup.Fatal(&w.Window, filterAndLogErrors("fatal:", err)...)
}

// SetLoading shows a spinning circle. It disables the window.
func (w *Window) SetLoading() {
	spinner := gtk.NewSpinner()
	spinner.SetSizeRequest(24, 24)
	spinner.SetHAlign(gtk.AlignCenter)
	spinner.SetVAlign(gtk.AlignCenter)
	spinner.Start()

	w.Window.SetChild(spinner)
	w.SetTitle("Loading")
	w.NotifyChild(true, func() { spinner.Stop() })
}

// NotifyChild calls f if the main window's child is changed. If once is true,
// then f is never called again.
func (w *Window) NotifyChild(once bool, f func()) {
	var childHandle glib.SignalHandle
	childHandle = w.Window.Connect("notify::child", func() {
		f()
		if once {
			w.Window.HandlerDisconnect(childHandle)
		}
	})
}

// SetSensitive sets whether or not the application's window is enabled.
func (w *Window) SetSensitive(sensitive bool) {
	w.Window.SetSensitive(sensitive)
}

// NewHeader creates a new header and puts it into the application window.
func (w *Window) NewHeader() *gtk.HeaderBar {
	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(true)
	w.Window.SetTitlebar(header)

	return header
}

// NewWindowHeader creates a new blank header.
func (w *Window) NewWindowHandle() *gtk.WindowHandle {
	header := gtk.NewWindowHandle()
	w.Window.SetTitlebar(header)

	return header
}

// SetTitle sets the application (and the main instance window)'s title.
func (w *Window) SetTitle(title string) {
	w.Window.SetTitle(SuffixedTitle(title))
}
