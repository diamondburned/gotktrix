package roomlist

import (
	"context"
	"log"
	"sort"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/section"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/pkg/errors"
)

// List describes a room list widget.
type List struct {
	*gtk.Box
	ctx  context.Context
	ctrl Controller

	outer    *adw.Bin
	inner    *gtk.Box // contains sections
	sections []*section.Section

	search string

	rooms   map[matrix.RoomID]*room.Room
	current matrix.RoomID
}

var listCSS = cssutil.Applier("roomlist-list", `
	.roomlist-list {
		background: @theme_base_color;
	}

	.roomlist-section {
		margin-bottom: 8px;
	}
	.roomlist-section list {
		background: inherit;
	}
	.roomlist-section list row:selected {
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
		Box:      gtk.NewBox(gtk.OrientationVertical, 0),
		outer:    adw.NewBin(),
		ctx:      ctx,
		ctrl:     ctrl,
		rooms:    make(map[matrix.RoomID]*room.Room, 100),
		sections: make([]*section.Section, 0, 10),
	}

	listCSS(roomList.outer)

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(roomList.outer)

	roomList.Append(scroll)
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

// AddRooms adds the rooms with the given IDs.
func (l *List) AddRooms(roomIDs []matrix.RoomID) {
	// Prefetch everything offline first.
	client := gotktrix.FromContext(l.ctx)
	state := client.Offline()
	retry := make([]matrix.RoomID, 0, len(roomIDs))

	defer l.refreshSections()

	for _, roomID := range roomIDs {
		// Ignore duplicate rooms.
		_, ok := l.rooms[roomID]
		if ok {
			continue
		}

		tagName := section.RoomTag(client, roomID)
		section := l.getOrCreateSection(tagName)

		r := room.AddTo(l.ctx, section, roomID)
		l.rooms[roomID] = r

		name, err := state.RoomName(roomID)
		if err != nil {
			// No known room names; delegate to later.
			retry = append(retry, roomID)
			// Don't bother fetching the avatar if we can't fetch the name.
			continue
		}

		// Update the room name.
		r.SetLabel(name)

		u, err := state.RoomAvatar(roomID)
		if err != nil {
			// No avatar found from querying; delegate.
			retry = append(retry, roomID)
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

func (l *List) getOrCreateSection(tag matrix.TagName) *section.Section {
	for _, sect := range l.sections {
		if sect.Tag() == tag {
			return sect
		}
	}

	sect := section.New(l.ctx, l, tag)
	sect.AddCSSClass("roomlist-section")
	l.sections = append(l.sections, sect)

	return sect
}

// sortcmp is a helper function that reverses the c comparison operation on j.
func sortcmp(i, j matrix.TagName, c func(matrix.TagName) bool) bool {
	if c(i) {
		return true
	}
	if c(j) {
		return false
	}
	return false
}

var matrixSectionOrder = map[string]int{
	"Favorites":     0,
	"People":        1,
	"Rooms":         2, // regular room
	"Low Priority":  3,
	"Server Notice": 4,
}

// refreshSections throws away the session box and recreates a new one from the
// internal list. It will sort the internal sections list.
func (l *List) refreshSections() {
	// Ensure that all old sections are removed from the old box.
	for _, s := range l.sections {
		s.Unparent()
	}

	sort.Slice(l.sections, func(i, j int) bool { // (i < j) -> (i before j)
		itag := l.sections[i].Tag()
		jtag := l.sections[j].Tag()

		iname := section.TagName(itag)
		jname := section.TagName(jtag)

		if section.TagEqNamespace(itag, jtag) {
			// Sort case insensitive.
			return sortutil.LessFold(iname, jname)
		}

		// User tags always go in front.
		if itag.Namespace("u") {
			return true
		}
		if jtag.Namespace("u") {
			return false
		}

		iord, iok := matrixSectionOrder[iname]
		jord, jok := matrixSectionOrder[jname]

		if iok && jok {
			return iord < jord
		}

		// Cannot compare tag, probably because the tag is neither a Matrix or
		// user tag. Put that tag in last.
		if iok {
			return true // jtag is not; itag in front.
		}
		if jok {
			return false // itag is not; jtag in front.
		}

		// Last resort: sort the tag namespace.
		return section.TagNamespace(itag) < section.TagNamespace(jtag)
	})

	l.inner = gtk.NewBox(gtk.OrientationVertical, 0)
	l.outer.SetChild(l.inner)

	// Insert the previous sections into the new box.
	for _, s := range l.sections {
		l.inner.Append(s)
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

		roomName, _ := client.RoomName(roomID)

		glib.IdleAdd(func() {
			if roomName != "" {
				room.SetLabel(roomName)
			}
		})
	}
}

// SetSelectedRoom sets the given room ID as the selected room row. It does not
// activate the room.
func (l *List) SetSelectedRoom(id matrix.RoomID) {
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
	srcRoom, ok := l.rooms[src]
	if !ok {
		// TODO: automatically create a new room so we can implement room
		// joining.
		return false
	}

	if !l.canMoveRoom(srcRoom, dst) {
		return false
	}

	oldTag := srcRoom.Section().Tag()
	newTag := dst.Tag()

	srcRoom.Move(dst)
	srcRoom.SetSensitive(false)

	go func() {
		defer glib.IdleAdd(func() { srcRoom.SetSensitive(true) })

		client := gotktrix.FromContext(l.ctx)

		// TODO: don't delete the room off of favorites.

		if err := client.TagDelete(src, oldTag); err != nil {
			app.Error(l.ctx, errors.Wrap(err, "failed to delete old room tag"))
			return
		}

		if err := client.TagAdd(src, newTag, matrix.Tag{}); err != nil {
			app.Error(l.ctx, errors.Wrap(err, "failed to add room tag"))
			return
		}

		glib.IdleAdd(func() { dst.InvalidateSort() })
	}()

	return true
}

// canMoveRoom checks that moving the given room to the given section is
// reasonable.
func (l *List) canMoveRoom(room *room.Room, sect *section.Section) bool {
	if room.IsIn(sect) {
		return false
	}

	isDirect := gotktrix.FromContext(l.ctx).Offline().IsDirect(room.ID)

	// Moving a non-DM room to the DM section is invalid.
	if !isDirect && sect.Tag() == section.DMSection {
		return false
	}

	// Moving the non-DM room to the regular rooms section is invalid.
	if isDirect && sect.Tag() == "" {
		return false
	}

	return true
}
