package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/about"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/app/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/app/blinker"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/messageview"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/app/userbutton"
	"github.com/diamondburned/gotktrix/internal/components/title"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
	"github.com/diamondburned/gotktrix/internal/config/prefs/prefui"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
	"github.com/pkg/errors"
	"golang.org/x/text/message"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"

	_ "github.com/diamondburned/gotktrix/internal/gtkutil/aggressivegc"
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
	glib.LogUseDefaultLogger()

	// Quit the application on a SIGINT.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Initialize translations and locales.
	ctx = locale.WithPrinter(ctx, locale.NewLocalPrinter(
		message.Catalog(locale.MustLoadLocales(locales)),
	))

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
		os.Exit(code)
	}
}

// TODO: allow multiple instances of the app? Application can provide a generic
// state API, and a package can be made to open a room from the given ID. To
// split a chat view into another window, simply open a new instance, ask it to
// open the room, and close it on our end.

func activate(ctx context.Context, gtkapp *gtk.Application) {
	adaptive.Init()

	// Load saved preferences.
	gtkutil.Async(ctx, func() func() {
		data, err := prefs.ReadSavedData()
		if err != nil {
			app.Error(ctx, errors.Wrap(err, "cannot read saved preferences"))
			return nil
		}

		return func() {
			if err := prefs.LoadData(data); err != nil {
				app.Error(ctx, errors.Wrap(err, "cannot load saved preferences"))
			}
		}
	})

	a := app.Wrap(gtkapp)
	w := a.Window()
	w.SetDefaultSize(700, 600)
	w.SetTitle("gotktrix")

	ctx = app.WithApplication(ctx, a)

	authAssistant := auth.Show(ctx)
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		client.Interceptor.AddIntercept(interceptHTTPLog)

		a.SetLoading()
		ctx := gotktrix.WithClient(ctx, client)

		// Making the blinker right here. We don't want to miss the first sync
		// once the screen becomes visible.
		m := manager{ctx: ctx}
		m.header.blinker = blinker.New(ctx)

		// Open the sync loop.
		syncbox.OpenThen(ctx, acc, func() { m.ready() })
	})
}

func interceptHTTPLog(r *http.Request, next func() error) error {
	if err := next(); err != nil && !errors.Is(err, context.Canceled) {
		log.Println("Matrix HTTP error:", err)
		return err
	}
	return nil
}

type manager struct {
	ctx context.Context

	window *gtk.Window
	fold   *adaptive.Fold

	header struct {
		*gtk.WindowHandle
		fold  *adaptive.Fold
		left  *gtk.Box
		ltext *gtk.Label
		right *gtk.Box
		rtext *title.Subtitle

		blinker *blinker.Blinker
	}

	roomList *roomlist.Browser
	msgView  *messageview.View
}

const minMessagesWidth = 400

var foldWidth = prefs.NewInt(275, prefs.IntMeta{
	Name:        "Sidebar Width",
	Section:     "Rooms",
	Description: "The width of the room list sidebar.",
	Min:         240,  // something refuses to go lower
	Max:         1000, // bruh
})

func (m *manager) ready() {
	a := app.FromContext(m.ctx)
	a.SetTitle("")

	m.window = a.Window()

	m.roomList = roomlist.New(m.ctx, m)
	m.roomList.SetVExpand(true)
	m.roomList.SetOverflow(gtk.OverflowHidden) // for shadow
	m.roomList.InvalidateRooms()

	welcome := adaptive.NewStatusPage()
	welcome.SetIconName("go-previous-symbolic")
	welcome.SetTitle(locale.Sprint(m.ctx, "Welcome"))
	welcome.SetDescriptionText(locale.Sprint(m.ctx, "Choose a room on the left panel."))

	m.msgView = messageview.New(m.ctx, m)
	m.msgView.SetPlaceholder(welcome)

	m.fold = adaptive.NewFold(gtk.PosLeft)
	m.fold.SetWidthFunc(m.width)
	m.fold.SetSideChild(m.roomList)
	m.fold.SetChild(m.msgView)

	a.Window().SetChild(m.fold)

	userID := gotktrix.FromContext(m.ctx).UserID
	username, _, _ := userID.Parse()

	m.header.ltext = gtk.NewLabel(username)
	m.header.ltext.AddCSSClass("app-username")
	m.header.ltext.SetTooltipText(string(userID))
	m.header.ltext.SetEllipsize(pango.EllipsizeEnd)
	m.header.ltext.SetHExpand(true)
	m.header.ltext.SetXAlign(0)

	roomSearch := gtk.NewToggleButton()
	roomSearch.SetIconName("system-search-symbolic")
	roomSearch.SetTooltipText(locale.S(m.ctx, "Search Room"))
	roomSearch.AddCSSClass("room-search-button")
	roomSearch.SetVAlign(gtk.AlignCenter)

	// Keep the button updated when the user activates search without it.
	roomSearchBar := m.roomList.SearchBar()
	roomSearchBar.Connect("notify::search-mode-enabled", func() {
		roomSearch.SetActive(roomSearchBar.SearchMode())
	})
	// Reveal or close the search bar when the button is toggled.
	roomSearch.ConnectClicked(func() {
		roomSearchBar.SetSearchMode(roomSearch.Active())
	})

	user := userbutton.NewToggle(m.ctx)
	user.SetTooltipText(locale.S(m.ctx, "Menu"))
	user.SetVAlign(gtk.AlignCenter)
	user.SetPopoverFunc(func(popover *gtk.PopoverMenu) {
		popover.SetParent(m.header.left)
		popover.SetPosition(gtk.PosBottom)
		popover.SetHasArrow(false)
		popover.SetSizeRequest(m.header.left.AllocatedWidth()-20, -1)
	})
	user.SetMenuFunc(func() []gtkutil.PopoverMenuItem {
		return []gtkutil.PopoverMenuItem{
			gtkutil.MenuSeparator(locale.S(m.ctx, "Me")),
			gtkutil.MenuItem(locale.S(m.ctx, "Custom _Emojis"), "win.user-emojis"),
			gtkutil.MenuSeparator(""),
			gtkutil.MenuItem(locale.S(m.ctx, "_Preferences"), "app.preferences"),
			gtkutil.MenuItem(locale.S(m.ctx, "_About"), "app.about"),
			gtkutil.MenuItem(locale.S(m.ctx, "_Quit"), "app.quit"),
		}
	})

	m.header.left = gtk.NewBox(gtk.OrientationHorizontal, 0)
	m.header.left.AddCSSClass("left-header")
	m.header.left.AddCSSClass("titlebar")
	m.header.left.Append(gtk.NewWindowControls(gtk.PackStart))
	m.header.left.Append(user)
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
	m.header.right.Append(m.header.blinker)
	m.header.right.Append(gtk.NewWindowControls(gtk.PackEnd))

	m.header.fold = adaptive.NewFold(gtk.PosLeft)
	m.header.fold.SetHExpand(true)
	m.header.fold.SetWidthFunc(m.width)
	m.header.fold.SetSideChild(m.header.left)
	m.header.fold.SetChild(m.header.right)

	foldWidth.SubscribeWidget(m.window, func() {
		width := foldWidth.Value()
		thres := width + minMessagesWidth
		m.fold.SetFoldThreshold(thres)
		m.fold.SetFoldWidth(width)
		m.header.fold.SetFoldThreshold(thres)
		m.header.fold.SetFoldWidth(width)
	})

	unfold.ConnectFold(m.fold)
	unfold.ConnectFold(m.header.fold)
	adaptive.BindFolds(m.fold, m.header.fold)

	m.header.WindowHandle = a.NewWindowHandle()
	m.header.SetChild(m.header.fold)

	gtkutil.BindActionMap(a.Window(), map[string]func(){
		"app.preferences": func() { prefui.ShowDialog(m.ctx) },
		"app.about":       func() { about.Show(m.ctx) },
		"app.quit":        func() { a.Quit() },
		"win.user-emojis": func() { emojiview.ForUser(m.ctx) },
	})
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
		m.header.rtext.SetSubtitle(page.RoomTopic())
	})
	m.SetSelectedRoom(id)
}

// SetSelectedRoom sets the given room ID as the selected room row. It does not
// activate the room. It exists solely as a callback for tabs.
func (m *manager) SetSelectedRoom(id matrix.RoomID) {
	m.roomList.SetSelectedRoom(id)
}

// ForwardTypingTo returns the message view's composer if there's one. Typing
// events on the room list that are uncaught will go into the composer.
func (m *manager) ForwardTypingTo() gtk.Widgetter {
	if current := m.msgView.Current(); current != nil {
		return current.Composer.Input()
	}
	return nil
}
