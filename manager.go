package main

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/title"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotktrix/internal/app/blinker"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/messageview"
	"github.com/diamondburned/gotktrix/internal/app/messageview/msgnotify"
	"github.com/diamondburned/gotktrix/internal/app/roomlist"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/app/userbutton"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/matrix"
)

type manager struct {
	ctx context.Context

	header struct {
		*gtk.WindowHandle
		fold  *adaptive.Fold
		left  *gtk.Box
		ltext *gtk.Label
		right *gtk.Box
		rtext *title.Subtitle

		blinker *blinker.Blinker
	}

	fold     *adaptive.Fold
	roomList *roomlist.Browser
	msgView  *messageview.View

	unbindLastRoom func()
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
	w := app.WindowFromContext(m.ctx)
	w.SetTitle("")

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
	m.fold.SetWidthFunc(w.AllocatedWidth)
	m.fold.SetSideChild(m.roomList)
	m.fold.SetChild(m.msgView)

	w.SetChild(m.fold)

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
	roomSearchBar.NotifyProperty("search-mode-enabled", func() {
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
			gtkutil.MenuItem(locale.S(m.ctx, "_Logs"), "app.logs"),
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
	m.header.fold.SetWidthFunc(w.AllocatedWidth)
	m.header.fold.SetSideChild(m.header.left)
	m.header.fold.SetChild(m.header.right)

	foldWidth.SubscribeWidget(w, func() {
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

	m.header.WindowHandle = w.NewWindowHandle()
	m.header.SetChild(m.header.fold)

	gtkutil.BindActionMap(w, map[string]func(){
		"win.user-emojis": func() { emojiview.ForUser(m.ctx) },
	})

	gtkutil.BindSubscribe(w, func() func() {
		return msgnotify.StartNotify(m.ctx, "app.open-room")
	})
}

func (m *manager) SearchRoom(name string) {
	m.roomList.Search(name)
}

func (m *manager) OpenRoom(id matrix.RoomID) {
	if m.unbindLastRoom != nil {
		m.unbindLastRoom()
		m.unbindLastRoom = nil
	}

	m.msgView.OpenRoom(id)
	m.SetSelectedRoom(id)

	rm := m.roomList.Room(id)

	// Slight side effect when doing this: if the room gets pushed outside the
	// visible section, then the information won't be updated until it's
	// revealed.
	m.unbindLastRoom = gtkutil.FuncBatcher(
		rm.NotifyName(func(_ context.Context, state room.State) {
			app.SetTitle(m.ctx, state.Name)
			m.header.rtext.SetTitle(state.Name)
		}),
		rm.NotifyTopic(func(_ context.Context, state room.State) {
			m.header.rtext.SetSubtitle(state.Topic)
		}),
	)
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
