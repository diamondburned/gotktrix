package section

import (
	"context"
	"log"
	"sort"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/diamondburned/gotrix/matrix"
)

var messageOnly = prefs.NewBool(false, prefs.PropMeta{
	Name:        "Sort Messages Only",
	Section:     "Rooms",
	Description: "Only sort rooms when there are new messages instead of any events.",
})

// SortSections sorts the given list of sections in a user-friendly way.
func SortSections(sections []*Section) {
	sort.Slice(sections, func(i, j int) bool {
		return lessTag(sections[i], sections[j])
	})
}

// (i < j) -> (i before j)
func lessTag(isect, jsect *Section) bool {
	itag := isect.Tag()
	jtag := jsect.Tag()

	if TagEqNamespace(itag, jtag) {
		// Sort case insensitive.
		return sortutil.LessFold(isect.tagName, jsect.tagName)
	}

	// User tags always go in front.
	if itag.HasNamespace("u") {
		return true
	}
	if jtag.HasNamespace("u") {
		return false
	}

	iord, iok := MatrixSectionOrder[itag]
	jord, jok := MatrixSectionOrder[jtag]

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
	return TagNamespace(itag) < TagNamespace(jtag)
}

// Controller describes the parent widget that Section controls.
type Controller interface {
	OpenRoom(matrix.RoomID)
	OpenRoomInTab(matrix.RoomID)

	// RoomIsVisible returns true if the given room should be visible.
	RoomIsVisible(matrix.RoomID) bool
	// IsSearching returns true if the user is searching for any room. This will
	// cause the section to not collapse.
	IsSearching() bool

	// VAdjustment returns the vertical scroll adjustment of the parent
	// controller. If not in list, return nil.
	VAdjustment() *gtk.Adjustment

	// MoveRoomToSection moves a room to another section. The method is expected
	// to verify that the moving is valid.
	MoveRoomToSection(src matrix.RoomID, dst *Section) bool
	// MoveRoomToTag moves the room with the given ID to the given tag name. A
	// new section must be created if needed.
	MoveRoomToTag(src matrix.RoomID, tag matrix.TagName) bool
}

const nMinified = 8

// Section is a room section, such as People or Favorites.
type Section struct {
	*gtk.Box
	ctx  context.Context
	ctrl Controller

	listBox *gtk.ListBox
	minify  *minifyButton

	rooms  map[matrix.RoomID]*room.Room
	hidden map[*room.Room]struct{}

	comparer Comparer

	selected *room.Room
	tagName  string

	// filtered is true if we're currently filtering out any rooms, either
	// because we're searching or we're displaying a space. This causes the
	// minifier to not work, because we don't keep track of filtered rooms.
	//
	// Ideally, we should just use a set and keep track of filtered rooms, which
	// will guarantee that this works.
	filtered bool
}

func acquireConfig(ctx context.Context, uID matrix.UserID) *app.State {
	return app.AcquireState(ctx, "sections", gotktrix.Base64UserID(uID), "state.json")
}

// New creates a new deactivated section.
func New(ctx context.Context, ctrl Controller, tag matrix.TagName) *Section {
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionSingle)
	list.SetActivateOnSingleClick(true)

	if vadj := ctrl.VAdjustment(); vadj != nil {
		list.SetAdjustment(vadj)
	}

	minify := newMinifyButton(ctx, true)
	minify.Hide()

	inner := gtk.NewBox(gtk.OrientationVertical, 0)
	inner.Append(list)
	inner.Append(minify)

	client := gotktrix.FromContext(ctx)
	cfg := acquireConfig(ctx, client.UserID)

	var reveal bool
	if !cfg.Get(string(tag), &reveal) {
		reveal = true
	}

	rev := gtk.NewRevealer()
	rev.SetRevealChild(reveal)
	rev.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	rev.SetChild(inner)
	rev.NotifyProperty("reveal-child", func() {
		if rev.RevealChild() {
			cfg.Set(string(tag), nil)
		} else {
			cfg.Set(string(tag), false)
		}
	})

	name := TagName(ctx, tag)

	btn := newRevealButton(rev, name)
	btn.SetHasFrame(false)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(btn)
	box.Append(rev)
	box.SetVisible(false)

	s := Section{
		Box:     box,
		ctx:     ctx,
		ctrl:    ctrl,
		minify:  minify,
		rooms:   make(map[matrix.RoomID]*room.Room),
		hidden:  make(map[*room.Room]struct{}),
		listBox: list,
		tagName: name,
	}

	gtkutil.BindActionMap(btn, map[string]func(){
		"roomsection.change-sort":  nil,
		"roomsection.show-preview": nil,
	})

	gtkutil.BindRightClick(btn, func() {
		box := gtk.NewBox(gtk.OrientationVertical, 0)
		box.Append(s.sortByBox())

		popover := gtk.NewPopover()
		popover.AddCSSClass("section-popover")
		popover.SetSizeRequest(gtkutil.PopoverWidth, -1)
		popover.SetPosition(gtk.PosBottom)
		popover.SetParent(btn)
		popover.SetChild(box)
		gtkutil.PopupFinally(popover)
	})

	minify.SetFunc(func() int {
		if s.filtered || len(s.rooms) <= nMinified {
			// hide the minify button
			return cannotMinify
		}
		return s.NHidden()
	})
	minify.ConnectClicked(func() {
		if minify.IsMinified() {
			s.Minimize()
		} else {
			s.Expand()
		}
	})

	s.listBox.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		ctrl.OpenRoom(matrix.RoomID(row.Name()))
	})

	s.comparer = *NewComparer(client.Offline(), SortActivity, tag)

	s.listBox.SetSortFunc(func(i, j *gtk.ListBoxRow) int {
		return s.comparer.Compare(matrix.RoomID(i.Name()), matrix.RoomID(j.Name()))
	})

	s.listBox.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
		visible := ctrl.RoomIsVisible(matrix.RoomID(row.Name()))
		if !visible {
			s.filtered = true
		}
		// Set this so we can count it later.
		row.SetVisible(visible)
		return visible
	})

	// default drag-and-drop mode.
	drop := gtk.NewDropTarget(glib.TypeString, gdk.ActionMove)
	drop.ConnectDrop(func(v glib.Value, _, _ float64) bool {
		srcID, ok := roomIDFromValue(&v)
		if !ok {
			return false
		}

		return s.ctrl.MoveRoomToSection(srcID, &s)
	})
	s.listBox.AddController(drop)

	// Re-sort if this is changed.
	messageOnly.SubscribeWidget(s, func() { s.InvalidateSort() })

	return &s
}

func roomIDFromValue(v *glib.Value) (matrix.RoomID, bool) {
	vstr, ok := v.GoValue().(string)
	if !ok {
		log.Printf("erroneous value not of type string, but %T", v.GoValue())
		return "", false
	}

	return matrix.RoomID(vstr), true
}

// Tag returns the tag name of this section.
func (s *Section) Tag() matrix.TagName {
	return s.comparer.Tag
}

func (s *Section) sortByBox() gtk.Widgetter {
	header := gtk.NewLabel(locale.S(s.ctx, "Sort by"))
	header.SetXAlign(0)
	header.SetAttributes(textutil.Attrs(
		pango.NewAttrWeight(pango.WeightBold),
	))

	radio := gtkutil.RadioData{
		Current: 1,
		Options: []string{
			locale.S(s.ctx, "Name (A-Z)"),
			locale.S(s.ctx, "Activity"),
		},
	}

	switch s.comparer.Mode {
	case SortName:
		radio.Current = 0
	case SortActivity:
		radio.Current = 1
	}

	b := gtk.NewBox(gtk.OrientationVertical, 0)
	b.Append(header)
	b.Append(gtkutil.NewRadioButtons(radio, func(i int) {
		switch i {
		case 0:
			s.SetSortMode(SortName)
		case 1:
			s.SetSortMode(SortActivity)
		}
	}))

	return b
}

// OpenRoom calls the parent controller's.
func (s *Section) OpenRoom(id matrix.RoomID) { s.ctrl.OpenRoom(id) }

// OpenRoomInTab calls the parent controller's.
func (s *Section) OpenRoomInTab(id matrix.RoomID) { s.ctrl.OpenRoomInTab(id) }

// MoveRoomToTag calls the parent controller's.
func (s *Section) MoveRoomToTag(src matrix.RoomID, tag matrix.TagName) bool {
	return s.ctrl.MoveRoomToTag(src, tag)
}

// SetSortMode sets the sorting mode for each room.
func (s *Section) SetSortMode(mode SortMode) {
	s.comparer.Mode = mode
	s.InvalidateSort()
}

// SortMode returns the section's current sort mode.
func (s *Section) SortMode() SortMode {
	return s.comparer.Mode
}

// Unselect unselects the list of the current section.
func (s *Section) Unselect() {
	if s.selected != nil {
		// Mark the row as inactive.
		s.selected.SetActive(false)
		s.selected = nil
	}

	s.listBox.UnselectAll()
}

// Select selects the room with the given ID. If an unknown ID is given, then
// the function panics.
func (s *Section) Select(id matrix.RoomID) {
	rm, ok := s.rooms[id]
	if !ok {
		log.Panicln("selecting unknown room", id)
	}

	rm.SetActive(true)
	s.selected = rm
	s.listBox.SelectRow(rm.ListBoxRow)
}

// HasRoom returns true if the section contains the given room.
func (s *Section) HasRoom(id matrix.RoomID) bool {
	_, ok := s.rooms[id]
	return ok
}

// Insert adds a room.
func (s *Section) Insert(room *room.Room) {
	if r, ok := s.rooms[room.ID]; ok {
		s.listBox.Remove(r.ListBoxRow)
		delete(s.rooms, room.ID)
	}

	room.ListBoxRow.SetName(string(room.ID))
	s.listBox.Insert(room.ListBoxRow, -1)

	s.rooms[room.ID] = room
	delete(s.hidden, room)

	if len(s.rooms) > nMinified && s.minify.IsMinified() {
		s.Minimize()
		s.minify.Invalidate()
	}

	s.invalidateVisibility()
}

// Remove removes the given room from the list.
func (s *Section) Remove(room *room.Room) {
	rm, ok := s.rooms[room.ID]
	if !ok {
		return
	}

	s.listBox.Remove(room.ListBoxRow)
	delete(s.hidden, rm)
	delete(s.rooms, room.ID)
	s.Reminify()

	s.invalidateVisibility()
}

// Changed reorders the given room specifically.
func (s *Section) Changed(room *room.Room) {
	s.comparer.InvalidateRoomCache()
	s.ReminifyAfter(func() { room.ListBoxRow.Changed() })
	s.invalidateVisibility()
}

// InvalidateSort invalidates the section's sort. This should be called if any
// room inside the section has been changed.
func (s *Section) InvalidateSort() {
	s.comparer.InvalidateRoomCache()
	s.ReminifyAfter(func() { s.listBox.InvalidateSort() })
}

// InvalidateFilter invalidates the filter.
func (s *Section) InvalidateFilter() {
	s.filtered = false
	s.ReminifyAfter(func() { s.listBox.InvalidateFilter() })
	s.invalidateVisibility()
}

func (s *Section) invalidateVisibility() {
	for _, room := range s.rooms {
		if room.Visible() {
			s.SetVisible(true)
			return
		}
	}
	// No visible room, so hide it.
	s.SetVisible(false)
}

// Reminify restores the minified state.
func (s *Section) Reminify() {
	s.ReminifyAfter(nil)
}

// ReminifyAfter restores the minified state only after executing after. If the
// section is not minified, then after is executed immediately. If after is nil,
// then it does the same thing as Reminify does.
func (s *Section) ReminifyAfter(after func()) {
	if !s.minify.IsMinified() || len(s.rooms) < nMinified {
		if after != nil {
			after()
		}
		s.minify.Invalidate()
		s.invalidateVisibility()
		return
	}

	s.expand()

	if after != nil {
		after()
	}

	if !s.filtered {
		s.Minimize()
	}

	s.minify.Invalidate()
	s.invalidateVisibility()
}

// NHidden returns the number of hidden rooms.
func (s *Section) NHidden() int {
	return len(s.hidden)
}

// Minimize minimizes the section to only show 8 entries.
func (s *Section) Minimize() {
	s.minify.SetMinified(true)

	if len(s.rooms) < nMinified {
		return
	}

	// Remove the rooms in backwards order so the list doesn't cascade back.
	for i := len(s.rooms) - 1; i >= nMinified; i-- {
		row := s.listBox.RowAtIndex(i)
		if row == nil {
			// This shouldn't happen.
			continue
		}

		// If the room isn't visible, then it's probably filtered away. Don't
		// add the room into the hidden map, since it'll mess up NHidden.
		if !row.Visible() {
			continue
		}

		room := s.roomRow(row)

		if _, ok := s.hidden[room]; !ok {
			row.SetVisible(false)
			s.hidden[room] = struct{}{}
		}
	}

	s.minify.Invalidate()
}

func (s *Section) roomRow(row *gtk.ListBoxRow) *room.Room {
	name := row.Name()

	room, ok := s.rooms[matrix.RoomID(name)]
	if !ok {
		log.Panicln("room ID", name, "missing in registry")
	}

	return room
}

// Expand makes the section display all rooms inside it.
func (s *Section) Expand() {
	s.minify.SetMinified(false)
	s.expand()
	s.minify.Invalidate()
}

func (s *Section) expand() {
	for r := range s.hidden {
		r.SetVisible(true)
		delete(s.hidden, r)
	}
}
