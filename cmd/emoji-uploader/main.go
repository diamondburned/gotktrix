package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/app/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
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

// TODO: allow multiple instances of the app? Application can provide a generic
// state API, and a package can be made to open a room from the given ID. To
// split a chat view into another window, simply open a new instance, ask it to
// open the room, and close it on our end.

func activate(gtkapp *gtk.Application) {
	app := app.Wrap(gtkapp)
	app.Window().SetTitle("Emoji Uploader")
	app.Window().Show()

	authAssistant := auth.New(app.Window())
	authAssistant.Show()
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		app.UseClient(client)

		go func() {
			popup := syncbox.Open(app, acc)
			popup.QueueSetLabel("Getting rooms...")

			rooms, err := client.Rooms()
			if err != nil {
				app.Fatal(err)
				return
			}

			glib.IdleAdd(func() {
				app := &application{Application: app}
				app.ready(rooms)
				popup.Close()
			})
		}()
	})
}

type application struct {
	*app.Application
	flap   *adw.Flap
	emojis *emojiview.View
}

func (app *application) ready(rooms []matrix.RoomID) {
	list := roomlist.New(app)
	list.AddRooms(rooms)

	listScroll := gtk.NewScrolledWindow()
	listScroll.SetHExpand(false)
	listScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	listScroll.SetChild(list)

	app.emojis = emojiview.NewForUser(app)
	app.emojis.SetHExpand(true)

	emojiScroll := gtk.NewScrolledWindow()
	emojiScroll.SetSizeRequest(250, -1)
	emojiScroll.SetHExpand(true)
	emojiScroll.SetChild(app.emojis)

	app.flap = adw.NewFlap()
	app.flap.SetFlap(listScroll)
	app.flap.SetContent(emojiScroll)
	app.flap.SetSwipeToOpen(true)
	app.flap.SetSwipeToClose(true)
	app.flap.SetFoldPolicy(adw.FlapFoldPolicyAuto)
	app.flap.SetTransitionType(adw.FlapTransitionTypeOver)
	app.flap.SetSeparator(gtk.NewSeparator(gtk.OrientationVertical))

	unflap := gtk.NewButtonFromIconName("document-properties-symbolic")
	unflap.InitiallyUnowned.Connect("clicked", func() {
		app.flap.SetRevealFlap(!app.flap.RevealFlap())
	})

	app.Window().SetChild(app.flap)
	app.Header().PackStart(unflap)
}

func (app *application) OpenRoom(id matrix.RoomID) {
	if app.emojis != nil {
		// Ensure that background work in the previous room is stopped.
		app.emojis.Stop()
	}

	app.emojis = emojiview.NewForRoom(app, id)
	app.emojis.SetHExpand(true)

	app.flap.SetContent(app.emojis)
}
