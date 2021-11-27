package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/app/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/app/blinker"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/messageview"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/selfbar"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"

	_ "github.com/diamondburned/gotktrix/internal/gtkutil/aggressivegc"
)

var _ = cssutil.WriteCSS(`
	.selfbar-bar {
		min-height: 46px;
		border-top: 1px solid @borders;
	}

	.left-sidebar {
		border-right: 1px solid @borders;
	}

	/* Use a border-bottom for this instead of border-top so the typing overlay
	 * can work properly.
	 */
	.messageview-rhs  .messageview-box > overlay {
		border-bottom: 1px solid @borders;
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
	adaptive.Init()

	a := app.Wrap(gtkapp)
	a.Window().SetDefaultSize(700, 600)
	a.Window().SetTitle("gotktrix")

	ctx = app.WithApplication(ctx, a)
	ctx = locale.WithLocalPrinter(ctx)

	authAssistant := auth.Show(ctx)
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		a.SetLoading()
		ctx := gotktrix.WithClient(ctx, client)

		gtkutil.Async(ctx, func() func() {
			popup := syncbox.Open(ctx, acc)
			popup.QueueSetLabel(locale.Sprint(ctx, "Getting rooms..."))

			rooms, err := client.Rooms()
			if err != nil {
				app.Fatal(ctx, err)
				return nil
			}

			return func() {
				m := manager{ctx: ctx}
				m.ready(rooms)
			}
		})
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
	self.SetVExpand(false)
	self.AddButton(locale.Sprint(m.ctx, "User Emojis"), func() {
		emojiview.ForUser(m.ctx)
	})

	leftBox := gtk.NewBox(gtk.OrientationVertical, 0)
	leftBox.AddCSSClass("left-sidebar")
	leftBox.SetSizeRequest(250, -1)
	leftBox.SetOverflow(gtk.OverflowHidden) // need this for box-shadow
	leftBox.SetHExpand(false)
	leftBox.Append(m.roomList)
	leftBox.Append(self)

	welcome := adaptive.NewStatusPage()
	welcome.SetIconName("go-previous-symbolic")
	welcome.SetTitle(locale.Sprint(m.ctx, "Welcome"))
	welcome.SetDescriptionText(locale.Sprint(m.ctx, "Choose a room on the left panel."))

	m.msgView = messageview.New(m.ctx, m)
	m.msgView.SetPlaceholder(welcome)

	fold := adaptive.NewFold(gtk.PosLeft)
	// GTK's awful image scaling requires us to do this. It might be a good idea
	// to implement a better image view that doesn't resize as greedily.
	fold.SetFoldThreshold(650)
	fold.SetFoldWidth(200)
	fold.SetSideChild(leftBox)
	fold.SetChild(m.msgView)

	unfold := adaptive.NewFoldRevealButton()
	unfold.ConnectFold(fold)

	a := app.FromContext(m.ctx)
	a.SetTitle("")
	a.Window().SetChild(fold)
	a.Header().PackStart(unfold)
	a.Header().PackEnd(blinker.New(m.ctx))
}

func (m *manager) OpenRoom(id matrix.RoomID) {
	name, _ := gotktrix.FromContext(m.ctx).Offline().RoomName(id)
	log.Println("opening room", name)

	m.msgView.OpenRoom(id)
	m.SetSelectedRoom(id)
}

/*
func (m *manager) OpenRoomInTab(id matrix.RoomID) {
	name, _ := gotktrix.FromContext(m.ctx).Offline().RoomName(id)
	log.Println("opening room", name, "in new tab")

	m.msgView.OpenRoomInNewTab(id)
	m.SetSelectedRoom(id)
}
*/

func (m *manager) SetSelectedRoom(id matrix.RoomID) {
	m.roomList.SetSelectedRoom(id)
}
