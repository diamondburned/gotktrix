package main

import (
	"context"
	"embed"
	"log"
	"net/http"

	"github.com/diamondburned/adaptive"
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

var _ = cssutil.WriteCSS(`
	windowhandle .adaptive-sidebar-revealer {
		background: none;
	}

	windowhandle,
	.composer,
	.roomlist-spaces-revealer > * {
		min-height: 46px;
	}

	.roomlist-spaces-revealer > * {
		border: none;
		border-top: 1px solid @borders;
	}

	/* Use a border-bottom for this instead of border-top so the typing overlay
	 * can work properly. */
	.messageview-rhs .messageview-box > overlay {
		border-bottom: 1px solid @borders;
	}

	.roomlist-spaces-revealer {
		box-shadow: 0 0 8px 0px rgba(0, 0, 0, 0.35);
	}

	.adaptive-sidebar-revealer > * {
		border-right: 1px solid @borders;
	}

	.adaptive-avatar label {
		background-color: mix(@theme_fg_color, @theme_bg_color, 0.75);
	}

	.left-header,
	.right-header .subtitle-title {
		font-weight: 600;
	}

	.left-header,
	.right-header {
		padding-right: 6px;
	}

	.left-header {
		padding-left: 6px;
		border-top-right-radius: 0;
	}

	.right-header {
		padding-left: 12px;
		border-top-left-radius: 0;
	}

	.right-header .subtitle {
		padding: 0px 0px;
		min-height: 46px;
	}

	.right-header .subtitle-subtitle {
		margin-top: -10px;
	}

	.right-header .adaptive-sidebar-reveal-button button {
		margin: 0 2px;
		margin-right: 12px;
	}

	.left-header  button:not(.userbutton-toggle),
	.right-header button {
		min-width:  32px; /* keep these in sync wcith room.AvatarSize */
		min-height: 32px;
		padding: 0;
	}

	.left-header > .app-username {
		margin: 0 4px;
	}

	/* Fix a quirk to do with the Default theme. */
	.titlebar box {
		opacity: initial;
	}
`)

//go:embed locales
var locales embed.FS

func main() {
	// Initialize translations and locales.
	ctx := locale.WithPrinter(context.Background(), locale.NewLocalPrinter(
		message.Catalog(locale.MustLoadLocales(locales)),
	))

	app := app.New(ctx, "com.github.diamondburned.gotktrix", "gotktrix")
	app.ConnectActivate(func() { activate(app.Context()) })
	app.RunMain()
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
