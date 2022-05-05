package space

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/section"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

// List describes a room list showing rooms within a space (or no space, i.e.
// the global space).
type List struct {
	*gtk.Box

	ctx  context.Context
	ctrl Controller

	SearchBar *gtk.SearchBar
	search    string

	scroll *gtk.ScrolledWindow
	outer  *adaptive.Bin
	inner  *gtk.Box // contains sections

	sections []*section.Section

	space spaceState
	rooms map[matrix.RoomID]*room.Room
}

// Controller describes the controller requirement.
type Controller interface {
	// OpenRoom opens the given room. The function should call back
	// List.SetSelectedRoom.
	OpenRoom(matrix.RoomID)
	// ForwardTypingTo returns the widget that typing events that are uncaught
	// in the List should be forwarded to.
	ForwardTypingTo() gtk.Widgetter
}

// RoomTabOpener can optionally be implemented by Application.
type RoomTabOpener interface {
	OpenRoomInTab(matrix.RoomID)
}

var listCSS = cssutil.Applier("space-list", `
	.space-list {
		background: @theme_base_color;
	}
	.space-section {
		margin-bottom: 8px;
	}
	.space-section list {
		background: inherit;
	}
	.space-reorderactions {
		color: @theme_selected_bg_color;
	}
	.space-reorderactions button {
		margin-left: 6px;
	}
`)

// New creates a new room list widget.
func New(ctx context.Context, ctrl Controller) *List {
	l := List{
		Box:      gtk.NewBox(gtk.OrientationVertical, 0),
		outer:    adaptive.NewBin(),
		ctx:      ctx,
		ctrl:     ctrl,
		rooms:    make(map[matrix.RoomID]*room.Room, 100),
		sections: make([]*section.Section, 0, 10),
	}

	listCSS(l.outer)

	l.scroll = gtk.NewScrolledWindow()
	l.scroll.SetVExpand(true)
	l.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	l.scroll.SetChild(l.outer)

	searchEntry := gtk.NewSearchEntry()
	searchEntry.SetHExpand(true)
	searchEntry.SetObjectProperty("placeholder-text", locale.S(ctx, "Search Rooms..."))
	searchEntry.ConnectSearchChanged(func() { l.Search(searchEntry.Text()) })

	l.SearchBar = gtk.NewSearchBar()
	l.SearchBar.AddCSSClass("space-search")
	l.SearchBar.ConnectEntry(&searchEntry.Editable)
	l.SearchBar.SetSearchMode(false)
	l.SearchBar.SetShowCloseButton(false)
	l.SearchBar.SetChild(searchEntry)
	l.SearchBar.NotifyProperty("search-mode-enabled", func() {
		if !l.SearchBar.SearchMode() {
			l.Search("")
		}
	})

	l.Append(l.SearchBar)
	l.Append(l.scroll)

	gtkutil.ForwardTypingFunc(l.scroll, func() gtk.Widgetter {
		return ctrl.ForwardTypingTo()
	})

	l.space = newSpaceState(l.InvalidateFilter)

	return &l
}

// SpaceID returns the ID of the currently displayed space. If it's empty, then
// the list will show all rooms, i.e. no filtering is done.
func (l *List) SpaceID() matrix.RoomID {
	return l.space.id
}

// SetSpaceID sets the space room ID of the list. This causes the list to filter
// out all rooms, only leaving behind rooms that belong to the given space.
// Sections that don't have any rooms after filtering will be hidden.
func (l *List) SetSpaceID(spaceID matrix.RoomID) {
	l.space.update(l.ctx, spaceID)
}

// VAdjustment returns the list's ScrolledWindow's vertical adjustment for
// scrolling.
func (l *List) VAdjustment() *gtk.Adjustment {
	return l.scroll.VAdjustment()
}

// RoomIsVisible returns true if the room with the given ID should be visible.
func (l *List) RoomIsVisible(roomID matrix.RoomID) bool {
	if l.search != "" {
		room, ok := l.rooms[roomID]
		if !ok {
			return false
		}
		if !sortutil.ContainsFold(room.Name, l.search) {
			return false
		}
	}

	if l.space.id != "" {
		if !l.space.children.has(roomID) {
			return false
		}
	}

	return true
}

// IsSearching returns true if the user is searching for rooms.
func (l *List) IsSearching() bool { return l.search != "" }

// Search searches for a room of the given name.
func (l *List) Search(str string) {
	l.search = str
	l.InvalidateFilter()
}

// InvalidateFilter invalidates all sections' filters.
func (l *List) InvalidateFilter() {
	for _, s := range l.sections {
		s.InvalidateFilter()
	}
}

// Room gets the room with the given ID, or nil if the room is unknown.
func (l *List) Room(id matrix.RoomID) *room.Room {
	return l.rooms[id]
}

// AddRoom adds the room into the given list.
func (l *List) AddRoom(roomID matrix.RoomID) {
	_, ok := l.rooms[roomID]
	if ok {
		return
	}

	client := gotktrix.FromContext(l.ctx).Offline()
	tagName := section.RoomTag(client, roomID)
	section := l.getOrCreateSection(tagName)

	l.rooms[roomID] = room.AddTo(l.ctx, section, roomID)
}

func (l *List) getOrCreateSection(tag matrix.TagName) *section.Section {
	for _, sect := range l.sections {
		if sect.Tag() == tag {
			return sect
		}
	}

	sect := section.New(l.ctx, l, tag)
	sect.AddCSSClass("space-section")
	l.sections = append(l.sections, sect)

	return sect
}

// InvalidateSections throws away the session box and recreates a new one from
// the internal list. It will sort the internal sections list.
func (l *List) InvalidateSections() {
	// Ensure that all old sections are removed from the old box.
	for _, s := range l.sections {
		s.Unparent()
	}

	section.SortSections(l.sections)

	l.inner = gtk.NewBox(gtk.OrientationVertical, 0)
	l.outer.SetChild(l.inner)

	// Insert the previous sections into the new box.
	for _, s := range l.sections {
		l.inner.Append(s)
	}
}

// SetSelectedRoom sets the given room ID as the selected room row. It does not
// activate the room.
func (l *List) SetSelectedRoom(id matrix.RoomID) {
	for _, sect := range l.sections {
		sect.Unselect()

		if sect.HasRoom(id) {
			sect.Select(id)
		}
	}
}

// OpenRoom opens the given room.
func (l *List) OpenRoom(id matrix.RoomID) {
	l.ctrl.OpenRoom(id)
}

// OpenRoomInTab opens the given room in a new tab.
func (l *List) OpenRoomInTab(id matrix.RoomID) {
	if opener, ok := l.ctrl.(RoomTabOpener); ok {
		opener.OpenRoomInTab(id)
	} else {
		l.ctrl.OpenRoom(id)
	}
}

// MoveRoomToTag moves the room to the new tag.
func (l *List) MoveRoomToTag(src matrix.RoomID, tag matrix.TagName) bool {
	oldOrder := -1.0
	if room, ok := l.rooms[src]; ok {
		oldOrder = room.Order()
	}

	section := l.getOrCreateSection(tag)

	if !l.MoveRoomToSection(src, section) {
		// Undo appending.
		l.sections[len(l.sections)-1] = nil
		l.sections = l.sections[:len(l.sections)-1]
		return false
	}

	// Restore the room's order number, if any.
	if oldOrder != -1 {
		l.rooms[src].SetOrder(oldOrder)
	}

	l.InvalidateSections()
	return true
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

	newTag := dst.Tag()

	srcRoom.Move(dst)
	srcRoom.SetSensitive(false)

	go func() {
		defer glib.IdleAdd(func() {
			dst.InvalidateSort()
			srcRoom.SetSensitive(true)
		})

		client := gotktrix.FromContext(l.ctx)

		var oldTags map[matrix.TagName]matrix.Tag

		e, err := client.RoomEvent(src, event.TypeTag)
		if err == nil {
			oldTags = e.(*event.TagEvent).Tags
		}

		omitNamespaces := func(nsps ...string) bool {
			if err := omitNamespaces(client, src, oldTags, nsps); err != nil {
				app.Error(l.ctx, errors.Wrap(err, "failed to delete old room tag"))
				return false
			}
			return true
		}

		switch {
		case newTag.HasNamespace("m"), section.TagIsIntern(newTag):
			// New tag has the Matrix namespace. Remove existing tags w/ the
			// Matrix namespace, since those conflict. Because we also
			// prioritize user namespaces over Matrix's, we also have to remove
			// them.
			if !omitNamespaces("m", "u") {
				return
			}
		// Other tags not in the Matrix namespace can co-exist.
		case newTag.HasNamespace("u"):
			// We have to be careful though, because the user has no control
			// over the deterministic process of sorting tags, so we only keep 1
			// user tag.
			if !omitNamespaces("u") {
				return
			}
		}

		// Don't add internal tags.
		if !section.TagIsIntern(newTag) {
			if err := client.TagAdd(src, newTag, matrix.Tag{}); err != nil {
				app.Error(l.ctx, errors.Wrap(err, "failed to add room tag"))
				return
			}
		}

		if err := client.UpdateRoomTags(src); err != nil {
			app.Error(l.ctx, errors.Wrap(err, "failed to update tag state"))
			return
		}
	}()

	return true
}

func omitNamespaces(
	c *gotktrix.Client, room matrix.RoomID,
	oldTags map[matrix.TagName]matrix.Tag, nsps []string) error {

	for _, nsp := range nsps {
		for name := range oldTags {
			if !name.HasNamespace(nsp) {
				continue
			}
			if err := c.TagDelete(room, name); err != nil {
				return errors.Wrap(err, "failed to delete old room tag")
			}
			delete(oldTags, name)
		}
	}

	return nil
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
	if isDirect && sect.Tag() == section.RoomsSection {
		return false
	}

	return true
}
