package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/app/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/app/messageview"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/selfbar"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/locale"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"

	_ "github.com/diamondburned/gotktrix/internal/gtkutil/aggressivegc"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

var _ = cssutil.WriteCSS(`
	.selfbar-bar, .composer {
		min-height: 46px;
	}
`)

func main() {
	glib.LogUseDefaultLogger()

	// Quit the application on a SIGINT.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	app := gtk.NewApplication(config.AppIDDot("gotktrix"), 0)
	app.Connect("activate", func() { activate(ctx, app) })

	go func() {
		<-ctx.Done()
		// Quit with high priority.
		glib.IdleAddPriority(coreglib.PriorityHigh, func() { app.Quit() })
	}()

	// Ensure the app quits on a panic.
	defer app.Quit()

	if code := app.Run(os.Args); code > 0 {
		log.Println("exit status", code)
		os.Exit(code)
	}
}

// TODO: allow multiple instances of the app? Application can provide a generic
// state API, and a package can be made to open a room from the given ID. To
// split a chat view into another window, simply open a new instance, ask it to
// open the room, and close it on our end.

func activate(ctx context.Context, gtkapp *gtk.Application) {
	a := app.Wrap(gtkapp)
	a.Window().SetDefaultSize(800, 600)
	a.Window().SetTitle("gotktrix")
	a.Window().Show()

	ctx = app.WithApplication(ctx, a)
	ctx = locale.WithLocalPrinter(ctx)

	authAssistant := auth.New(ctx)
	authAssistant.Show()
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		ctx := gotktrix.WithClient(ctx, client)

		go func() {
			popup := syncbox.Open(ctx, acc)
			popup.QueueSetLabel(locale.Sprint(ctx, "Getting rooms..."))

			rooms, err := client.Rooms()
			if err != nil {
				app.Fatal(ctx, err)
				return
			}

			glib.IdleAdd(func() {
				m := manager{ctx: ctx}
				m.ready(rooms)
				popup.Close()
			})
		}()
	})
}

type manager struct {
	ctx context.Context

	roomList *roomlist.List
	msgView  *messageview.View
}

func (m *manager) ready(rooms []matrix.RoomID) {
	m.roomList = roomlist.New(m.ctx, m)
	m.roomList.SetVExpand(true)
	m.roomList.AddRooms(rooms)

	self := selfbar.New(m.ctx, m)
	self.Invalidate()

	leftBox := gtk.NewBox(gtk.OrientationVertical, 0)
	leftBox.SetSizeRequest(250, -1)
	leftBox.SetOverflow(gtk.OverflowHidden) // need this for box-shadow
	leftBox.SetHExpand(false)
	leftBox.Append(m.roomList)
	leftBox.Append(self)

	welcome := adw.NewStatusPage()
	welcome.SetIconName("go-previous-symbolic")
	welcome.SetTitle(locale.Sprint(m.ctx, "Welcome"))
	welcome.SetDescription(locale.Sprint(m.ctx, "Choose a room on the left panel."))

	m.msgView = messageview.New(m.ctx, m)
	m.msgView.SetPlaceholder(welcome)

	flap := adw.NewFlap()
	flap.SetFlap(leftBox)
	flap.SetContent(m.msgView)
	flap.SetSwipeToOpen(true)
	flap.SetSwipeToClose(true)
	flap.SetFoldPolicy(adw.FlapFoldPolicyAuto)
	flap.SetTransitionType(adw.FlapTransitionTypeOver)
	flap.SetSeparator(gtk.NewSeparator(gtk.OrientationVertical))

	const (
		revealedIcon   = "pan-start-symbolic"
		unrevealedIcon = "document-properties-symbolic"
	)

	unflap := gtk.NewButtonFromIconName(unrevealedIcon)

	updateUnflap := func(flap bool) {
		if flap {
			unflap.SetIconName(revealedIcon)
		} else {
			unflap.SetIconName(unrevealedIcon)
		}
	}
	updateUnflap(flap.RevealFlap())

	unflap.Connect("clicked", func() {
		reveal := !flap.RevealFlap()
		flap.SetRevealFlap(reveal)
		updateUnflap(reveal)
	})

	a := app.FromContext(m.ctx)
	a.Window().SetChild(flap)
	a.Header().PackStart(unflap)
}

func (m *manager) OpenRoom(id matrix.RoomID) {
	name, _ := gotktrix.FromContext(m.ctx).Offline().RoomName(id)
	log.Println("opening room", name)

	m.msgView.OpenRoom(id)
	m.SetSelectedRoom(id)
}

func (m *manager) OpenRoomInTab(id matrix.RoomID) {
	name, _ := gotktrix.FromContext(m.ctx).Offline().RoomName(id)
	log.Println("opening room", name, "in new tab")

	m.msgView.OpenRoomInNewTab(id)
	m.SetSelectedRoom(id)
}

func (m *manager) SetSelectedRoom(id matrix.RoomID) {
	m.roomList.SetSelectedRoom(id)
}
