package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/app/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
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

func activate(gtkapp *gtk.Application) {
	app := app.Wrap(gtkapp)
	app.Window.SetTitle("Emoji Uploader")
	app.Window.Show()

	authAssistant := auth.New(&app.Window.Window)
	authAssistant.Show()
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		app.Client = client

		go func() {
			popup := syncbox.Open(app, acc)
			popup.QueueSetLabel("Getting rooms...")

			rooms, err := client.Rooms()
			if err != nil {
				app.Fatal(err)
				return
			}

			glib.IdleAdd(func() {
				ready(app, rooms)
				popup.Close()
			})
		}()
	})
}

func ready(app *app.Application, rooms []matrix.RoomID) {
	list := roomlist.New(app.Client)
	list.AddRooms(rooms)

	listScroll := gtk.NewScrolledWindow()
	listScroll.SetHExpand(false)
	listScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	listScroll.SetChild(list)

	emojis := emojiview.NewForUser(app)
	emojis.SetHExpand(true)

	emojiScroll := gtk.NewScrolledWindow()
	emojiScroll.SetSizeRequest(250, -1)
	emojiScroll.SetHExpand(true)
	emojiScroll.SetChild(emojis)

	flap := adw.NewFlap()
	flap.SetFlap(listScroll)
	flap.SetContent(emojiScroll)
	flap.SetSwipeToOpen(true)
	flap.SetSwipeToClose(true)
	flap.SetFoldPolicy(adw.FlapFoldPolicyAuto)
	flap.SetTransitionType(adw.FlapTransitionTypeOver)
	flap.SetSeparator(gtk.NewSeparator(gtk.OrientationVertical))

	list.OnRoom(func(roomID matrix.RoomID) {
		if emojis != nil {
			// Ensure that background work in the previous room is stopped.
			emojis.Stop()
		}

		emojis = emojiview.NewForRoom(app, roomID)
		emojis.SetHExpand(true)

		flap.SetContent(emojis)
	})

	unflap := gtk.NewButtonFromIconName("document-properties-symbolic")
	unflap.InitiallyUnowned.Connect("clicked", func() {
		flap.SetRevealFlap(!flap.RevealFlap())
	})

	app.Window.SetChild(flap)
	app.Header.PackStart(unflap)
}
