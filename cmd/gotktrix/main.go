package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/app/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/app/messageview"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/selfbar"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/gotk3/gotk3/glib"
)

func main() {
	runtime.LockOSThread()

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
	app.Window().SetDefaultSize(800, 600)
	app.Window().SetTitle("gotktrix")
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
				app := application{Application: app}
				app.ready(rooms)
				popup.Close()
			})
		}()
	})
}

type application struct {
	*app.Application
	roomList *roomlist.List
	msgView  *messageview.View
}

var (
	_ roomlist.Application = (*application)(nil)
)

func (app *application) ready(rooms []matrix.RoomID) {
	app.roomList = roomlist.New(app)
	app.roomList.AddRooms(rooms)

	listScroll := gtk.NewScrolledWindow()
	listScroll.SetVExpand(true)
	listScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	listScroll.SetChild(app.roomList)

	self := selfbar.New(app.Application)
	self.Invalidate()

	leftBox := gtk.NewBox(gtk.OrientationVertical, 0)
	leftBox.SetOverflow(gtk.OverflowHidden) // need this for box-shadow
	leftBox.SetHExpand(false)
	leftBox.Append(listScroll)
	leftBox.Append(self)

	welcome := adw.NewStatusPage()
	welcome.SetIconName("go-previous-symbolic")
	welcome.SetTitle("Welcome")
	welcome.SetDescription("Choose a room on the left panel.")

	app.msgView = messageview.New(app)
	app.msgView.SetPlaceholder(welcome)

	flap := adw.NewFlap()
	flap.SetFlap(leftBox)
	flap.SetContent(app.msgView)
	flap.SetSwipeToOpen(true)
	flap.SetSwipeToClose(true)
	flap.SetFoldPolicy(adw.FlapFoldPolicyAuto)
	flap.SetTransitionType(adw.FlapTransitionTypeOver)
	flap.SetSeparator(gtk.NewSeparator(gtk.OrientationVertical))

	unflap := gtk.NewButtonFromIconName("document-properties-symbolic")
	unflap.InitiallyUnowned.Connect("clicked", func() {
		flap.SetRevealFlap(!flap.RevealFlap())
	})

	app.Window().SetChild(flap)
	app.Header().PackStart(unflap)
}

func (app *application) OpenRoom(id matrix.RoomID) {
	name, _ := app.Client().Offline().RoomName(id)
	log.Println("opening room", name)

	app.msgView.OpenRoom(id)
	app.SetSelectedRoom(id)
}

func (app *application) OpenRoomInTab(id matrix.RoomID) {
	name, _ := app.Client().Offline().RoomName(id)
	log.Println("opening room", name, "in new tab")

	app.msgView.OpenRoomInNewTab(id)
	app.SetSelectedRoom(id)
}

func (app *application) SetSelectedRoom(id matrix.RoomID) {
	app.roomList.SetSelectedRoom(id)
}
