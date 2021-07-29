package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/davecgh/go-spew/spew"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/auth"
	"github.com/diamondburned/gotktrix/internal/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/emojis"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/roomsort"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/gotk3/gotk3/glib"
)

func main() {
	app := gtk.NewApplication(config.AppIDDot("emoji-uploader"), 0)
	app.Connect("activate", activate)

	// Quit the application on a SIGINT.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go func() {
		<-ctx.Done()
		// Quit with high priority.
		glib.IdleAddPriority(glib.PRIORITY_HIGH, func() { app.Quit() })
	}()

	if code := app.Run(os.Args); code > 0 {
		log.Println("exit status", code)
		os.Exit(code)
	}
}

func activate(app *gtk.Application) {
	cssutil.ApplyGlobalCSS()

	// flap := adw.NewFlap()

	window := gtk.NewApplicationWindow(app)
	window.SetDefaultSize(450, 300)
	window.SetTitle("Emoji Uploader")
	window.Show()

	fiddle := func(client *gotktrix.Client, acc *auth.Account) {
		syncbox.Open(window, acc, client)

		e, err := client.WaitForUserEvent(context.Background(), emojis.UserEmotesEventType)
		if err != nil {
			errpopup.ShowFatal(&window.Window, []error{err})
			return
		}

		spew.Dump(e)

		roomIDs, err := roomsort.SortedRooms(client, roomsort.SortAlphabetically)
		if err != nil {
			errpopup.ShowFatal(&window.Window, []error{err})
			return
		}

		for _, roomID := range roomIDs {
			name, err := client.RoomName(roomID)
			if err != nil {
				log.Printf("room %q -> <unknown name: %v>", roomID, err)
			} else {
				log.Printf("room %q -> %q", roomID, name)
			}
		}
	}

	authAssistant := auth.New(&window.Window)
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		go fiddle(client, acc)
	})
	authAssistant.Show()
}
