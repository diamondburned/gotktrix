package roomlist

import (
	"log"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/section"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// List describes a room list widget.
type List struct {
	*gtk.Box
	app    Application
	client *gotktrix.Client

	section struct {
		rooms  *section.Section
		people *section.Section
	}

	sections []*section.Section
	search   string

	rooms   map[matrix.RoomID]*room.Room
	current matrix.RoomID
}

var listCSS = cssutil.Applier("roomlist-list", `
	.roomlist-list {
		background: @theme_base_color;
	}
	.roomlist-list > * {
		margin-bottom: 8px;
	}
	.roomlist-list list {
		background: inherit;
	}
	.roomlist-list list row:selected {
		background-color: alpha(@accent_color, 0.2);
		color: mix(@accent_color, @theme_fg_color, 0.25);
	}
`)

// Application describes the application requirement.
type Application interface {
	app.Applicationer
	OpenRoom(matrix.RoomID)
}

// RoomTabOpener can optionally be implemented by Application.
type RoomTabOpener interface {
	OpenRoomInTab(matrix.RoomID)
}

// New creates a new room list widget.
func New(app Application) *List {
	roomList := List{
		Box:    gtk.NewBox(gtk.OrientationVertical, 0),
		app:    app,
		client: app.Client(),
		rooms:  make(map[matrix.RoomID]*room.Room),
		sections: []*section.Section{
			section.New("Rooms"),
			section.New("People"),
		},
	}

	roomList.section.rooms = roomList.sections[0]
	roomList.section.people = roomList.sections[1]

	for _, section := range roomList.sections {
		section.SetParentList(&roomList)
		roomList.Append(section)
	}

	listCSS(roomList)

	return &roomList
}

// Client returns the list's client.
func (l *List) Client() *gotktrix.Client { return l.client }

// Searching returns the string being searched.
func (l *List) Searching() string { return l.search }

// Search searches for a room of the given name.
func (l *List) Search(str string) {
	l.search = str

	for _, s := range l.sections {
		s.InvalidateFilter()
	}
}

// // PrependSection prepends the given section into the list.
// func (l *List) PrependSection(s *Section) {
// 	l.Prepend(s)
// 	l.sections = append([]*Section{s}, l.sections...)
// 	s.SetParentList(l)
// }

// // AppendSection appends the given section into the list.
// func (l *List) AppendSection(s *Section) {
// 	l.Append(s)
// 	l.sections = append(l.sections, s)
// 	s.SetParentList(l)
// }

// AddRooms adds the rooms with the given IDs.
func (l *List) AddRooms(roomIDs []matrix.RoomID) {
	// Prefetch everything offline first.
	state := l.client.Offline()
	retry := make([]matrix.RoomID, 0, len(roomIDs))

	for _, roomID := range roomIDs {
		// Ignore duplicate rooms.
		_, ok := l.rooms[roomID]
		if ok {
			continue
		}

		var willRetry bool

		direct, ok := l.client.State.IsDirect(roomID)
		if !ok {
			// Delegate rooms that we're unsure if it's direct or not to later,
			// but still add it into the room list.
			retry = append(retry, roomID)
			willRetry = true
		}

		var r *room.Room
		if direct {
			r = room.AddTo(l.section.people, roomID)
		} else {
			r = room.AddTo(l.section.rooms, roomID)
		}

		// Register the room anyway.
		l.rooms[roomID] = r

		name, err := state.RoomName(roomID)
		if err != nil {
			// No known room names; delegate to later.
			if !willRetry {
				retry = append(retry, roomID)
			}
			// Don't bother fetching the avatar if we can't fetch the name.
			continue
		}

		// Update the room name.
		r.SetLabel(name)

		u, err := state.RoomAvatar(roomID)
		if err != nil {
			// No avatar found from querying; delegate.
			if !willRetry {
				retry = append(retry, roomID)
			}
			continue
		}

		if u != nil {
			r.SetAvatarURL(*u)
		}
	}

	if len(retry) > 0 {
		go func() { l.syncAddRooms(retry) }()
	}
}

func (l *List) syncAddRooms(roomIDs []matrix.RoomID) {
	for _, roomID := range roomIDs {
		room, ok := l.rooms[roomID]
		if !ok {
			continue
		}

		// TODO: don't fetch avatar twice.
		u, _ := l.client.RoomAvatar(roomID)
		if u != nil {
			room.SetAvatarURL(*u)
		}

		// Double-check that the room is in the correct section.
		move := room.IsIn(l.section.rooms) && l.client.IsDirect(roomID)

		roomName, _ := l.client.RoomName(roomID)

		glib.IdleAdd(func() {
			if roomName != "" {
				room.SetLabel(roomName)
			}

			if move {
				// Room is now direct after querying API; move it to the right
				// place.
				room.Move(l.section.people)
			}
		})
	}
}

// SetSelectedRoom sets the given room ID as the selected room row. It does not
// activate the room.
func (l *List) SetSelectedRoom(id matrix.RoomID) {
	// log.Println("marking-selecting room", id)
	for _, sect := range l.sections {
		if sect.HasRoom(id) {
			sect.Select(id)
			return
		}
	}
}

// OpenRoom opens the given room.
func (l *List) OpenRoom(id matrix.RoomID) {
	l.setRoom(id)
	l.app.OpenRoom(id)
}

// OpenRoomInTab opens the given room in a new tab.
func (l *List) OpenRoomInTab(id matrix.RoomID) {
	l.setRoom(id)

	if opener, ok := l.app.(RoomTabOpener); ok {
		opener.OpenRoomInTab(id)
	} else {
		l.app.OpenRoom(id)
	}
}

func (l *List) setRoom(id matrix.RoomID) {
	l.current = id

	if _, ok := l.rooms[id]; !ok {
		log.Panicf("room %q not in registry", id)
	}

	for _, s := range l.sections {
		s.Unselect(l.current)
	}
}
