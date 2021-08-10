package roomlist

import (
	"context"
	"log"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/section"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/roomsort"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/pkg/errors"
)

// List describes a room list widget.
type List struct {
	*gtk.Box
	ctx  context.Context
	ctrl Controller

	sectionBox *gtk.Box
	sections   []*section.Section
	section    struct {
		rooms  *section.Section
		people *section.Section
	}

	search string

	rooms   map[matrix.RoomID]*room.Room
	current matrix.RoomID

	reorderAction *gtk.ActionBar
	reordering    *reorderingState
}

type reorderingState struct {
	state roomsort.RoomPositions
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

	.roomlist-reorderactions {
		color: @accent_color;
	}
	.roomlist-reorderactions button {
		margin-left: 6px;
	}
`)

// Controller describes the controller requirement.
type Controller interface {
	OpenRoom(matrix.RoomID)
}

// RoomTabOpener can optionally be implemented by Application.
type RoomTabOpener interface {
	OpenRoomInTab(matrix.RoomID)
}

// New creates a new room list widget.
func New(ctx context.Context, ctrl Controller) *List {
	roomList := List{
		Box:   gtk.NewBox(gtk.OrientationVertical, 0),
		ctx:   ctx,
		ctrl:  ctrl,
		rooms: make(map[matrix.RoomID]*room.Room),
	}

	roomList.sections = []*section.Section{
		section.New(ctx, &roomList, "Rooms"),
		section.New(ctx, &roomList, "People"),
	}

	roomList.section.rooms = roomList.sections[0]
	roomList.section.people = roomList.sections[1]

	sectionBox := gtk.NewBox(gtk.OrientationVertical, 0)
	listCSS(sectionBox)

	for _, section := range roomList.sections {
		sectionBox.Append(section)
	}

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(sectionBox)

	cancel := gtk.NewButtonFromIconName("process-stop-symbolic")
	cancel.Connect("clicked", roomList.stopOrdering)
	done := gtk.NewButtonFromIconName("object-select-symbolic")
	done.Connect("clicked", roomList.saveOrdering)

	roomList.reorderAction = gtk.NewActionBar()
	roomList.reorderAction.AddCSSClass("roomlist-reorderactions")
	roomList.reorderAction.SetRevealed(false)
	roomList.reorderAction.PackStart(gtk.NewLabel("Reorder rooms"))
	roomList.reorderAction.PackEnd(done)
	roomList.reorderAction.PackEnd(cancel)

	roomList.Append(scroll)
	roomList.Append(roomList.reorderAction)
	return &roomList
}

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
	client := gotktrix.FromContext(l.ctx)
	state := client.Offline()
	retry := make([]matrix.RoomID, 0, len(roomIDs))

	for _, roomID := range roomIDs {
		// Ignore duplicate rooms.
		_, ok := l.rooms[roomID]
		if ok {
			continue
		}

		var willRetry bool

		direct, ok := client.State.IsDirect(roomID)
		if !ok {
			// Delegate rooms that we're unsure if it's direct or not to later,
			// but still add it into the room list.
			retry = append(retry, roomID)
			willRetry = true
		}

		var r *room.Room
		if direct {
			r = room.AddTo(l.ctx, l.section.people, roomID)
		} else {
			r = room.AddTo(l.ctx, l.section.rooms, roomID)
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
	client := gotktrix.FromContext(l.ctx)

	for _, roomID := range roomIDs {
		room, ok := l.rooms[roomID]
		if !ok {
			continue
		}

		// TODO: don't fetch avatar twice.
		u, _ := client.RoomAvatar(roomID)
		if u != nil {
			room.SetAvatarURL(*u)
		}

		// Double-check that the room is in the correct section.
		move := room.IsIn(l.section.rooms) && client.IsDirect(roomID)

		roomName, _ := client.RoomName(roomID)

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
	l.ctrl.OpenRoom(id)
}

// OpenRoomInTab opens the given room in a new tab.
func (l *List) OpenRoomInTab(id matrix.RoomID) {
	l.setRoom(id)

	if opener, ok := l.ctrl.(RoomTabOpener); ok {
		opener.OpenRoomInTab(id)
	} else {
		l.ctrl.OpenRoom(id)
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

// MoveRoomToSection moves the room w/ the given ID to the given section. False
// is returend if the return doesn't make sense.
func (l *List) MoveRoomToSection(src matrix.RoomID, dst *section.Section) bool {
	if l.reordering != nil {
		// In room reordering mode, section moving is handled automatically.
		return false
	}

	srcRoom, ok := l.rooms[src]
	if !ok {
		// TODO: automatically create a new room so we can implement room
		// joining.
		return false
	}

	if !l.canMoveRoom(srcRoom, dst) {
		return false
	}

	srcRoom.Move(dst)
	return true
}

// canMoveRoom checks that moving the given room to the given section is
// reasonable.
func (l *List) canMoveRoom(room *room.Room, sect *section.Section) bool {
	if room.IsIn(sect) {
		return false
	}

	// DM check.
	direct := gotktrix.FromContext(l.ctx).Offline().IsDirect(room.ID)

	// Moving a non-DM room to the DM section is invalid.
	if !direct && sect == l.section.people {
		return false
	}
	// Moving the non-DM room to the regular rooms section OR the Low Priority
	// section is invalid.
	if direct && sect == l.section.rooms {
		// TODO: add Low Priority room section.
		return false
	}

	return true
}

// MoveRoom implements section.ParentList.
func (l *List) MoveRoom(src matrix.RoomID, dstRoom *room.Room, pos gtk.PositionType) bool {
	if l.reordering == nil {
		// Not in reordering mode; don't allow.
		return false
	}

	srcRoom, ok := l.rooms[src]
	if !ok {
		return false
	}

	dstSection, ok := dstRoom.Section().(*section.Section)
	if !ok {
		log.Printf("BUG: room has unknown section type %T", dstRoom.Section())
		return false
	}

	if !srcRoom.IsIn(dstSection) {
		if !l.canMoveRoom(srcRoom, dstSection) {
			return false
		}
		// Move should remove the room from the section.
		srcRoom.Move(dstSection)
	}

	var anchor roomsort.Anchor
	switch pos {
	case gtk.PosTop:
		anchor = roomsort.AnchorAbove
	case gtk.PosBottom:
		anchor = roomsort.AnchorBelow
	}

	l.reordering.state[src] = roomsort.RoomPosition{
		RelID:  dstRoom.ID,
		Anchor: anchor,
	}

	dstSection.UseRoomPositions(l.reordering.state)
	return true
}

// BeginReorderMode implements selfbar's.
func (l *List) BeginReorderMode() {
	if l.reordering != nil {
		// Already in reordering mode.
		return
	}

	l.AddCSSClass("reordering")

	client := gotktrix.FromContext(l.ctx).Offline()

	// Grab the current state.
	e, err := client.UserEvent(roomsort.RoomPositionEventType)
	if err != nil {
		e = roomsort.RoomPositionEvent{}
	}

	pos := e.(roomsort.RoomPositionEvent)
	if pos.Positions == nil {
		pos.Positions = make(roomsort.RoomPositions, 1)
	}

	l.reorderAction.SetRevealed(true)
	l.reordering = &reorderingState{
		state: pos.Positions,
	}

	for _, section := range l.sections {
		section.BeginReorderMode()
	}
}

func (l *List) stopOrdering() {
	if l.reordering == nil {
		return
	}

	l.RemoveCSSClass("reordering")
	l.reordering = nil
	l.reorderAction.SetRevealed(false)
}

func (l *List) saveOrdering() {
	if l.reordering == nil {
		// Not in reordering mode for some reason.
		return
	}

	ev := roomsort.RoomPositionEvent{
		Positions: l.reordering.state,
	}

	gotktrix.FromContext(l.ctx).AsyncSetConfig(ev, func(err error) {
		if err != nil {
			app.Error(l.ctx, errors.Wrap(err, "failed to update roomsort list"))
		}
	})

	// Update the order of the room.
	for _, section := range l.sections {
		section.EndReorderMode()
		section.InvalidateSort()
	}

	l.stopOrdering()
}
