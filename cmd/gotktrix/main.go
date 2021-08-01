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
	"github.com/diamondburned/gotktrix/internal/app/messageview"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/gotk3/gotk3/glib"
)

func main() {
	app := gtk.NewApplication(config.AppIDDot("gotktrix"), 0)
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
	app.Window.SetTitle("gotktrix")
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

	welcome := adw.NewStatusPage()
	welcome.SetIconName("go-previous-symbolic")
	welcome.SetTitle("Welcome")
	welcome.SetDescription("Choose a room on the left panel.")

	msgview := messageview.New(app)
	msgview.SetPlaceholder(welcome)

	flap := adw.NewFlap()
	flap.SetFlap(listScroll)
	flap.SetContent(msgview)
	flap.SetSwipeToOpen(true)
	flap.SetSwipeToClose(true)
	flap.SetFoldPolicy(adw.FlapFoldPolicyAuto)
	flap.SetTransitionType(adw.FlapTransitionTypeOver)
	flap.SetSeparator(gtk.NewSeparator(gtk.OrientationVertical))

	list.OnRoom(func(roomID matrix.RoomID) {
		msgview.OpenRoom(roomID)
	})

	unflap := gtk.NewButtonFromIconName("document-properties-symbolic")
	unflap.InitiallyUnowned.Connect("clicked", func() {
		flap.SetRevealFlap(!flap.RevealFlap())
	})

	app.Window.SetChild(flap)
	app.Header.PackStart(unflap)
}
