package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/app/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/app/blinker"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/messageview"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/selfbar"
	"github.com/diamondburned/gotktrix/internal/components/title"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"

	_ "github.com/diamondburned/gotktrix/internal/gtkutil/aggressivegc"
)

var _ = cssutil.WriteCSS(`
	windowhandle .adaptive-sidebar-revealer {
		background: none;
	}

	windowhandle, .selfbar-bar, .composer {
		min-height: 46px;
	}

	.adaptive-sidebar-revealer {
		border-right: 1px solid @borders;
	}

	.left-header, .right-header .subtitle-title {
		font-weight: 600;
	}

	.left-header, .right-header {
		padding-right: 6px;
	}

	.left-header {
		padding-left: 8px;
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

	.app-menu {
		margin-right: 6px;
	}

	.selfbar-bar {
		border-top: 1px solid @borders;
	}

	/* Use a border-bottom for this instead of border-top so the typing overlay
	 * can work properly. */
	.messageview-rhs .messageview-box > overlay {
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
	w := a.Window()
	w.SetDefaultSize(700, 600)
	w.SetTitle("gotktrix")

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

	window *gtk.Window
	fold   *adaptive.Fold
	unfold *adaptive.FoldRevealButton

	header struct {
		*gtk.WindowHandle
		fold  *adaptive.Fold
		left  *gtk.Box
		ltext *gtk.Label
		right *gtk.Box
		rtext *title.Subtitle
	}

	roomList *roomlist.List
	msgView  *messageview.View

	actions   *gio.SimpleActionGroup
	menuItems [][2]string // id->label
}

const (
	foldThreshold = 650
	foldWidth     = 250
)

func (m *manager) ready(rooms []matrix.RoomID) {
	a := app.FromContext(m.ctx)
	a.SetTitle("")

	m.window = a.Window()

	m.roomList = roomlist.New(m.ctx, m)
	m.roomList.SetVExpand(true)
	m.roomList.AddRooms(rooms)

	self := selfbar.New(m.ctx, m)
	self.SetVExpand(false)
	self.Invalidate()
	self.AddButton(locale.Sprint(m.ctx, "User Emojis"), func() {
		emojiview.ForUser(m.ctx)
	})

	leftBox := gtk.NewBox(gtk.OrientationVertical, 0)
	leftBox.AddCSSClass("left-sidebar")
	leftBox.SetOverflow(gtk.OverflowHidden) // need this for box-shadow
	leftBox.Append(m.roomList)
	leftBox.Append(self)

	welcome := adaptive.NewStatusPage()
	welcome.SetIconName("go-previous-symbolic")
	welcome.SetTitle(locale.Sprint(m.ctx, "Welcome"))
	welcome.SetDescriptionText(locale.Sprint(m.ctx, "Choose a room on the left panel."))

	m.msgView = messageview.New(m.ctx, m)
	m.msgView.SetPlaceholder(welcome)

	m.fold = adaptive.NewFold(gtk.PosLeft)
	m.fold.SetWidthFunc(m.width)
	// GTK's awful image scaling requires us to do this. It might be a good idea
	// to implement a better image view that doesn't resize as greedily.
	m.fold.SetFoldThreshold(foldThreshold)
	m.fold.SetFoldWidth(foldWidth)
	m.fold.SetSideChild(leftBox)
	m.fold.SetChild(m.msgView)

	a.Window().SetChild(m.fold)

	m.header.ltext = gtk.NewLabel("gotktrix")
	m.header.ltext.SetEllipsize(pango.EllipsizeEnd)
	m.header.ltext.SetHExpand(true)
	m.header.ltext.SetXAlign(0)

	roomSearch := gtk.NewToggleButton()
	roomSearch.SetIconName("system-search-symbolic")
	roomSearch.SetTooltipText(locale.S(m.ctx, "Search Room"))
	roomSearch.AddCSSClass("room-search-button")
	roomSearch.SetVAlign(gtk.AlignCenter)
	// Reveal or close the search bar when the button is toggled.
	roomSearch.ConnectClicked(func() {
		m.roomList.SearchBar.SetSearchMode(roomSearch.Active())
	})
	// Keep the button updated when the user activates search without it.
	m.roomList.SearchBar.Connect("notify::search-mode-enabled", func() {
		roomSearch.SetActive(m.roomList.SearchBar.SearchMode())
	})

	burger := gtk.NewToggleButton()
	burger.AddCSSClass("app-menu")
	burger.SetIconName("open-menu")
	burger.SetTooltipText(locale.S(m.ctx, "Menu"))
	burger.SetVAlign(gtk.AlignCenter)
	burger.ConnectClicked(func() {
		p := gtkutil.NewPopoverMenu(m.header.left, gtk.PosBottom, m.menuItems)
		// p.SetOffset(0, -4)
		// TODO: fix up the arrow to point to the button
		p.SetHasArrow(false)
		p.SetSizeRequest(230, -1)
		p.ConnectClosed(func() { burger.SetActive(false) })
		p.Popup()
	})

	m.header.left = gtk.NewBox(gtk.OrientationHorizontal, 0)
	m.header.left.AddCSSClass("left-header")
	m.header.left.AddCSSClass("titlebar")
	m.header.left.Append(gtk.NewWindowControls(gtk.PackStart))
	m.header.left.Append(burger)
	m.header.left.Append(m.header.ltext)
	m.header.left.Append(roomSearch)

	unfold := adaptive.NewFoldRevealButton()
	unfold.Button.SetVAlign(gtk.AlignCenter)
	unfold.Button.SetIconName("open-menu")
	unfold.Revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideRight)

	m.header.rtext = title.NewSubtitle()
	m.header.rtext.SetXAlign(0)
	m.header.rtext.SetHExpand(true)

	m.header.right = gtk.NewBox(gtk.OrientationHorizontal, 0)
	m.header.right.AddCSSClass("right-header")
	m.header.right.AddCSSClass("titlebar")
	m.header.right.Append(unfold)
	m.header.right.Append(m.header.rtext)
	m.header.right.Append(blinker.New(m.ctx))
	m.header.right.Append(gtk.NewWindowControls(gtk.PackEnd))

	m.header.fold = adaptive.NewFold(gtk.PosLeft)
	m.header.fold.SetHExpand(true)
	m.header.fold.SetWidthFunc(m.width)
	m.header.fold.SetFoldThreshold(foldThreshold)
	m.header.fold.SetFoldWidth(foldWidth)
	m.header.fold.SetSideChild(m.header.left)
	m.header.fold.SetChild(m.header.right)

	unfold.ConnectFold(m.fold)
	unfold.ConnectFold(m.header.fold)
	adaptive.BindFolds(m.fold, m.header.fold)

	m.header.WindowHandle = a.NewWindowHandle()
	m.header.SetChild(m.header.fold)

	m.actions = gio.NewSimpleActionGroup()
	a.Window().InsertActionGroup("app", m.actions)

	m.addAction("Preferences", func() {})
	m.addAction("About", func() {})
}

func (m *manager) addAction(label string, f func()) {
	// TODO: make abstraction for selfbar as well
	id := gtkutil.ActionID(label)
	m.actions.AddAction(gtkutil.ActionFunc(id, f))
	m.menuItems = append(m.menuItems, [2]string{label, "app." + id})
}

func (m *manager) width() int {
	return m.window.AllocatedWidth()
}

func (m *manager) SearchRoom(name string) {
	m.roomList.Search(name)
}

func (m *manager) OpenRoom(id matrix.RoomID) {
	page := m.msgView.OpenRoom(id)
	page.OnTitle(func(string) {
		app.SetTitle(m.ctx, page.RoomName())
		m.header.rtext.SetTitle(page.RoomName())
		m.header.rtext.SetSubtitle(firstLine(page.RoomTopic()))
	})
	m.SetSelectedRoom(id)
}

func firstLine(lines string) string {
	if lines == "" {
		return ""
	}
	return strings.SplitN(lines, "\n", 2)[0]
}

func (m *manager) SetSelectedRoom(id matrix.RoomID) {
	m.roomList.SetSelectedRoom(id)
}
