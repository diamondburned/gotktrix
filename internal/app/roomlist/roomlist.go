package roomlist

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/gotk3/gotk3/glib"
)

// List describes a room list widget.
type List struct {
	*gtk.Box
	client *gotktrix.Client

	section struct {
		rooms  *Section
		people *Section
	}

	sections []*Section
	search   string

	onRoom  func(matrix.RoomID)
	rooms   map[matrix.RoomID]Room
	current matrix.RoomID
}

var listCSS = cssutil.Applier("roomlist-list", `
	.roomlist-list {
		background: @theme_base_color;
	}
	.roomlist-list list {
		background: inherit;
	}
	.roomlist-list list row:selected {
		background-color: alpha(@accent_color, 0.2);
		color: mix(@accent_color, @theme_fg_color, 0.25);
	}
`)

// New creates a new room list widget.
func New(client *gotktrix.Client) *List {
	roomList := List{
		Box:    gtk.NewBox(gtk.OrientationVertical, 0),
		client: client,
		rooms:  make(map[matrix.RoomID]Room),
		sections: []*Section{
			NewSection("Rooms"),
			NewSection("People"),
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

func (l *List) Search(str string) {
	l.search = str

	for _, s := range l.sections {
		s.List.InvalidateFilter()
	}
}

// OnRoom sets the function to be called when a room is selected.
func (l *List) OnRoom(f func(matrix.RoomID)) {
	l.onRoom = f
}

// PrependSection prepends the given section into the list.
func (l *List) PrependSection(s *Section) {
	l.Prepend(s)
	l.sections = append([]*Section{s}, l.sections...)
	s.SetParentList(l)
}

// AppendSection appends the given section into the list.
func (l *List) AppendSection(s *Section) {
	l.Append(s)
	l.sections = append(l.sections, s)
	s.SetParentList(l)
}

// AddRooms adds the rooms with the given IDs.
func (l *List) AddRooms(roomIDs []matrix.RoomID) {
	// Prefetch everything offline first.
	state := l.client.WithContext(gotktrix.Cancelled())
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

		var r Room
		if direct {
			r = AddEmptyRoom(l.section.people, roomID)
		} else {
			r = AddEmptyRoom(l.section.rooms, roomID)
		}

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

		e, err := state.RoomState(roomID, event.TypeRoomAvatar, "")
		if err != nil {
			// No avatar found from querying; delegate.
			if !willRetry {
				retry = append(retry, roomID)
			}
			continue
		}

		if e != nil {
			avatarEv := e.(event.RoomAvatarEvent)
			url, _ := state.SquareThumbnail(avatarEv.URL, AvatarSize)
			imgutil.AsyncGET(context.TODO(), url, r.Avatar.SetCustomImage)
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
		e, err := l.client.RoomState(roomID, event.TypeRoomAvatar, "")
		if err == nil && e != nil {
			avatarEv := e.(event.RoomAvatarEvent)
			url, _ := l.client.SquareThumbnail(avatarEv.URL, AvatarSize)
			imgutil.AsyncGET(context.TODO(), url, room.Avatar.SetCustomImage)
		}

		// Double-check that the room is in the correct section.
		move := room.section == l.section.rooms && l.client.IsDirect(roomID)

		roomName, _ := l.client.RoomName(roomID)

		glib.IdleAdd(func() {
			if roomName != "" {
				room.SetLabel(roomName)
			}

			if move {
				// Room is now direct after querying API; move it to the right
				// place.
				room.move(l.section.people)
			}
		})
	}
}

func (l *List) setRoom(id matrix.RoomID) {
	l.current = id

	for _, s := range l.sections {
		s.Unselect(l.current)
	}

	if l.onRoom != nil {
		l.onRoom(id)
	}
}
