package roomlist

import (
	"context"
	"log"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/space"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/matrix"
)

// Browser describes a widget holding
type Browser struct {
	*gtk.Box
	list   *space.List
	spaces struct {
		*gtk.Revealer
		scroll *gtk.ScrolledWindow

		box     *gtk.Box
		buttons map[matrix.RoomID]spaceButton
	}

	ctx context.Context
}

var spacesCSS = cssutil.Applier("roomlist-spaces", `
	.roomlist-spaces {
		padding: 0 6px;
	}
	.roomlist-spaces > * {
		border-radius: 999px 999px;
		padding:    0;
		min-width:  0;
		min-height: 0;
	}
`)

var spacesRevealerCSS = cssutil.Applier("roomlist-spaces-revealer", ``)

// New creates a new spaces browser.
func New(ctx context.Context, ctrl space.Controller) *Browser {
	b := Browser{ctx: ctx}
	b.list = space.New(ctx, ctrl)
	b.list.SetVExpand(true)

	allRooms := NewAllRoomsButton(ctx)
	allRooms.SetActive(true)
	allRooms.ConnectClicked(func() { b.chooseSpace(allRooms) })

	b.spaces.box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	b.spaces.box.SetHAlign(gtk.AlignCenter)
	b.spaces.box.SetVAlign(gtk.AlignCenter)
	b.spaces.box.Append(allRooms)
	spacesCSS(b.spaces.box)

	b.spaces.buttons = make(map[matrix.RoomID]spaceButton, 1)
	b.spaces.buttons[""] = allRooms

	viewport := gtk.NewViewport(nil, nil)
	viewport.SetChild(b.spaces.box)
	viewport.SetHScrollPolicy(gtk.ScrollNatural)
	viewport.SetScrollToFocus(true)

	b.spaces.scroll = gtk.NewScrolledWindow()
	b.spaces.scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyNever)
	// This causes overflow.
	// b.spaces.scroll.SetPropagateNaturalWidth(true)
	b.spaces.scroll.SetChild(viewport)

	b.spaces.Revealer = gtk.NewRevealer()
	b.spaces.SetChild(b.spaces.scroll)
	b.spaces.SetRevealChild(false)
	b.spaces.SetTransitionType(gtk.RevealerTransitionTypeSlideUp)
	spacesRevealerCSS(b.spaces.Revealer)

	b.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	b.Box.Append(b.list)
	b.Box.Append(b.spaces)

	return &b
}

// InvalidateRooms refetches the room list and updates everything.
func (b *Browser) InvalidateRooms() {
	client := gotktrix.FromContext(b.ctx)

	roomIDs, _ := client.State.Rooms()
	if len(roomIDs) > 0 {
		state := gotktrix.FromContext(b.ctx).Offline()
		for _, roomID := range roomIDs {
			b.addRoom(roomID, state.RoomType(roomID))
		}

		b.list.InvalidateSections()
	}

	go func() {
		known := make(map[matrix.RoomID]bool, len(roomIDs))
		for _, id := range roomIDs {
			known[id] = true
		}

		roomIDs, _ := client.Client.Rooms()

		for _, roomID := range roomIDs {
			if known[roomID] {
				continue
			}

			delete(known, roomID)

			typ := client.RoomType(roomID)
			roomID := roomID

			glib.IdleAdd(func() {
				switch typ {
				case "":
					b.list.AddRoom(roomID)
				case "m.space":
					b.addSpace(roomID)
				}
			})
		}

		if len(known) > 0 {
			glib.IdleAdd(func() { b.list.InvalidateSections() })
		}
	}()
}

func (b *Browser) addRoom(roomID matrix.RoomID, typ string) {
	switch typ {
	case "":
		b.list.AddRoom(roomID)
	case "m.space":
		b.addSpace(roomID)
	default:
		log.Printf("room %s has unknown type %q", roomID, typ)
	}
}

func (b *Browser) addSpace(spaceID matrix.RoomID) {
	_, ok := b.spaces.buttons[spaceID]
	if ok {
		return
	}

	space := NewSpaceButton(b.ctx, spaceID)
	space.ConnectClicked(func() { b.chooseSpace(space) })

	b.spaces.buttons[spaceID] = space
	b.spaces.box.Append(space)

	b.spaces.SetRevealChild(true)
}

func (b *Browser) chooseSpace(chosen spaceButton) {
	for _, button := range b.spaces.buttons {
		// Force active when clicked.
		button.SetActive(button == chosen)
	}

	switch button := chosen.(type) {
	case *SpaceButton:
		b.list.SetSpaceID(button.SpaceID())
	case *AllRoomsButton:
		b.list.SetSpaceID("")
	}
}

// Search searches the room browwser for the given query.
func (b *Browser) Search(query string) {
	b.list.Search(query)
}

// SearchBar returns the list's search bar widget.
func (b *Browser) SearchBar() *gtk.SearchBar {
	return b.list.SearchBar
}

// SetSelectedRoom sets the given room ID as the selected room row. It does not
// activate the room.
func (b *Browser) SetSelectedRoom(id matrix.RoomID) {
	b.list.SetSelectedRoom(id)
}

// Room gets the room with the given ID, or nil if it's not known.
func (b *Browser) Room(id matrix.RoomID) *room.Room {
	return b.list.Room(id)
}
