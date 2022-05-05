package main

import (
	"context"
	"embed"
	"log"
	"net/http"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/components/prefui"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/about"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/app/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/app/blinker"
	"github.com/diamondburned/gotktrix/internal/app/messageview/msgnotify"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
	"golang.org/x/text/message"

	_ "github.com/diamondburned/gotkit/gtkutil/aggressivegc"
)

//go:embed styles/main.css
var loadedCss string
var _ = cssutil.WriteCSS(loadedCss)

//go:embed locales
var locales embed.FS

func main() {
	glib.LogUseDefaultLogger()

	// Initialize translations and locales.
	ctx := locale.WithPrinter(context.Background(), locale.NewLocalPrinter(
		message.Catalog(locale.MustLoadLocales(locales)),
	))

	app := app.New("com.github.diamondburned.gotktrix", "gotktrix")
	app.ConnectActivate(func() { activate(app.Context()) })
	app.RunMain(ctx)
}

// initialized is true if the global initializers are ran. We assume that
// globally, there's only ever 1 GApplication instance.
var initialized bool

// managers keeps track of all manager instances that are unique to each user.
var managers = map[matrix.UserID]*manager{}

// openRoom opens the room using the manager with the given user ID. If no user
// ID is given or if the user ID is not found, then the command is dropped.
func openRoom(cmd msgnotify.OpenRoomCommand) {
	manager, ok := managers[cmd.UserID]
	if !ok {
		log.Println("user ID", cmd.UserID, "not found")
		return
	}
	manager.OpenRoom(cmd.RoomID)
}

func activate(ctx context.Context) {
	a := app.FromContext(ctx)

	if !initialized {
		initialized = true

		adaptive.Init()

		// Load saved preferences.
		gtkutil.Async(ctx, func() func() {
			data, err := prefs.ReadSavedData(ctx)
			if err != nil {
				a.Error(errors.Wrap(err, "cannot read saved preferences"))
				return nil
			}

			return func() {
				if err := prefs.LoadData(data); err != nil {
					a.Error(errors.Wrap(err, "cannot load saved preferences"))
				}
			}
		})

		a.AddActions(map[string]func(){
			"app.preferences": func() { prefui.ShowDialog(ctx) },
			"app.about":       func() { about.Show(ctx) },
			"app.logs":        func() { logui.ShowDefaultViewer(ctx) },
			"app.quit":        func() { a.Quit() },
		})

		a.AddActionCallbacks(map[string]gtkutil.ActionCallback{
			"app.open-room": gtkutil.NewJSONActionCallback(openRoom),
		})
	}

	w := a.NewWindow()
	w.SetDefaultSize(700, 600)
	w.SetTitle("gotktrix")

	ctx = app.WithWindow(ctx, w)

	authAssistant := auth.Show(ctx)
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		ctx := gotktrix.WithClient(ctx, client)
		client.Interceptor.AddIntercept(interceptHTTPLog)

		// Making the blinker right here. We don't want to miss the first sync
		// once the screen becomes visible.
		m := manager{ctx: ctx}
		m.header.blinker = blinker.New(ctx)

		managers[client.UserID] = &m
		w.ConnectDestroy(func() { delete(managers, client.UserID) })

		// Open the sync loop.
		w.SetLoading()
		syncbox.OpenThen(ctx, acc, func() { m.ready() })
	})
}

func interceptHTTPLog(r *http.Request, next func() error) error {
	err := next()
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Println("Matrix HTTP error:", err)
	}
	return err
}
