package section

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/sortutil"
)

// SortSections sorts the given list of sections in a user-friendly way.
func SortSections(sections []*Section) {
	sort.Slice(sections, func(i, j int) bool {
		itag := sections[i].Tag()
		jtag := sections[j].Tag()

		return lessTag(itag, jtag)
	})
}

// (i < j) -> (i before j)
func lessTag(itag, jtag matrix.TagName) bool {
	if TagEqNamespace(itag, jtag) {
		iname := TagName(itag)
		jname := TagName(jtag)

		// Sort case insensitive.
		return sortutil.LessFold(iname, jname)
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

	// Searching returns the string being searched.
	Searching() string

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
	hidden map[*room.Room]bool

	comparer Comparer

	selected    *room.Room
	minified    bool
	showPreview bool
}

// New creates a new deactivated section.
func New(ctx context.Context, ctrl Controller, tag matrix.TagName) *Section {
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionSingle)
	list.SetActivateOnSingleClick(true)
	list.SetPlaceholder(gtk.NewLabel("No rooms yet..."))

	if vadj := ctrl.VAdjustment(); vadj != nil {
		list.SetAdjustment(vadj)
	}

	minify := newMinifyButton(true)
	minify.Hide()

	inner := gtk.NewBox(gtk.OrientationVertical, 0)
	inner.Append(list)
	inner.Append(minify)

	rev := gtk.NewRevealer()
	rev.SetRevealChild(true)
	rev.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	rev.SetChild(inner)

	btn := newRevealButton(rev, TagName(tag))
	btn.SetHasFrame(false)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(btn)
	box.Append(rev)

	s := Section{
		Box:         box,
		ctx:         ctx,
		ctrl:        ctrl,
		minify:      minify,
		rooms:       make(map[matrix.RoomID]*room.Room),
		hidden:      make(map[*room.Room]bool),
		listBox:     list,
		showPreview: true, // TODO config module
	}

	gtkutil.BindActionMap(btn, "roomsection", map[string]func(){
		"change-sort":  nil,
		"show-preview": nil,
	})

	gtkutil.BindRightClick(btn, func() {
		gtkutil.ShowPopoverMenuCustom(btn, gtk.PosBottom, []gtkutil.PopoverMenuItem{
			gtkutil.MenuWidget("roomsection.change-sort", s.sortByBox()),
			gtkutil.MenuSeparator("Appearance"),
			gtkutil.MenuWidget("roomsection.show-preview", s.showPreviewBox()),
		})
	})

	minify.OnToggled(func(minify bool) string {
		if !minify {
			s.Expand()
			return "Show less"
		}

		s.Minimize()
		return fmt.Sprintf("Show %d more", s.NHidden())
	})

	s.listBox.Connect("row-activated", func(list *gtk.ListBox, row *gtk.ListBoxRow) {
		ctrl.OpenRoom(matrix.RoomID(row.Name()))
	})

	client := gotktrix.FromContext(ctx)
	s.comparer = *NewComparer(client.Offline(), SortActivity, tag)

	s.listBox.SetSortFunc(func(i, j *gtk.ListBoxRow) int {
		return s.comparer.Compare(matrix.RoomID(i.Name()), matrix.RoomID(j.Name()))
	})

	s.listBox.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
		searching := ctrl.Searching()
		if searching == "" {
			return true
		}

		rm, ok := s.rooms[matrix.RoomID(row.Name())]
		if !ok {
			return false
		}

		return strings.Contains(rm.Name, searching)
	})

	// default drag-and-drop mode.
	drop := gtk.NewDropTarget(glib.TypeString, gdk.ActionMove)
	drop.Connect("drop", func(_ *gtk.DropTarget, v *glib.Value) bool {
		srcID, ok := roomIDFromValue(v)
		if !ok {
			return false
		}

		return s.ctrl.MoveRoomToSection(srcID, &s)
	})
	s.listBox.AddController(drop)

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

func (s *Section) showPreviewBox() gtk.Widgetter {
	check := gtk.NewCheckButtonWithLabel("Show Message Preview")
	check.Connect("toggled", func() {
		s.showPreview = check.Active()
		// Update all rooms individually. No magic here.
		for _, room := range s.rooms {
			room.SetShowMessagePreview(s.showPreview)
		}
	})

	return check
}

func (s *Section) sortByBox() gtk.Widgetter {
	header := gtk.NewLabel("Sort by")
	header.SetXAlign(0)
	header.SetAttributes(markuputil.Attrs(
		pango.NewAttrWeight(pango.WeightBold),
	))

	radio := gtkutil.RadioData{
		Current: 1,
		Options: []string{"Name (A-Z)", "Activity"},
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
	s.comparer = *NewComparer(gotktrix.FromContext(s.ctx).Offline(), mode, s.comparer.Tag)
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

	room.SetShowMessagePreview(s.showPreview)
	room.ListBoxRow.SetName(string(room.ID))
	s.listBox.Insert(room.ListBoxRow, -1)

	s.rooms[room.ID] = room
	s.hidden[room] = false

	if len(s.rooms) > nMinified && s.minified {
		s.Minimize()
		s.minify.Invalidate()
	}
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
}

// InvalidateSort invalidates the section's sort. This should be called if any
// room inside the section has been changed.
func (s *Section) InvalidateSort() {
	s.comparer.InvalidateRoomCache()
	s.ReminifyAfter(func() { s.listBox.InvalidateSort() })
}

// InvalidateFilter invalidates the filtler.
func (s *Section) InvalidateFilter() {
	s.ReminifyAfter(func() { s.listBox.InvalidateFilter() })
}

// Reminify restores the minified state.
func (s *Section) Reminify() {
	s.ReminifyAfter(nil)
}

// ReminifyAfter restores the minified state only after executing after. If the
// section is not minified, then after is executed immediately. If after is nil,
// then it does the same thing as Reminify does.
func (s *Section) ReminifyAfter(after func()) {
	if !s.minified || len(s.rooms) < nMinified {
		after()
		return
	}

	s.expand()

	if after != nil {
		after()
	}

	s.Minimize()
}

// NHidden returns the number of hidden rooms.
func (s *Section) NHidden() int {
	if !s.minified || len(s.rooms) <= nMinified {
		return 0
	}
	return len(s.rooms) - nMinified
}

// Minimize minimizes the section to only show 8 entries.
func (s *Section) Minimize() {
	s.minified = true

	if len(s.rooms) < nMinified {
		return
	}

	s.minify.Show()

	// Remove the rooms in backwards order so the list doesn't cascade back.
	for i := len(s.rooms) - 1; i >= nMinified; i-- {
		row := s.listBox.RowAtIndex(i)
		if row == nil {
			// This shouldn't happen.
			continue
		}

		room, ok := s.rooms[matrix.RoomID(row.Name())]
		if !ok {
			log.Panicln("room ID", row.Name(), "missing in registry")
		}

		if !s.hidden[room] {
			s.listBox.Remove(row)
			s.hidden[room] = true
		}
	}
}

// Expand makes the section display all rooms inside it.
func (s *Section) Expand() {
	s.minified = false
	s.expand()

	if len(s.rooms) > nMinified {
		s.minify.Show()
	}
}

func (s *Section) expand() {
	for r, hidden := range s.hidden {
		if hidden {
			s.listBox.Append(r.ListBoxRow)
			s.hidden[r] = false
		}
	}
}
