package section

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/roomsort"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

const nMinified = 8

// Controller describes the parent widget that Section controls.
type Controller interface {
	OpenRoom(matrix.RoomID)
	OpenRoomInTab(matrix.RoomID)

	// Searching returns the string being searched.
	Searching() string

	// MoveRoom moves the src room into where dst was accordingly to the
	// position type. The method is expected to also asynchronously save the
	// position into the server. True should be returned if the move is
	// successful.
	MoveRoom(src matrix.RoomID, dst *room.Room, pos gtk.PositionType) bool
	// MoveRoomToSection moves a room to another section. The method is expected
	// to verify that the moving is valid.
	MoveRoomToSection(src matrix.RoomID, dst *Section) bool
}

// Section is a room section, such as People or Favorites.
type Section struct {
	*gtk.Box
	ctx  context.Context
	ctrl Controller

	listBox *gtk.ListBox
	minify  *minifyButton

	rooms  map[matrix.RoomID]*room.Room
	hidden map[*room.Room]bool

	comparer *roomsort.Comparer
	current  matrix.RoomID

	sectionDrop *gtk.DropTarget // only for moving sections
	reordering  *gtk.DropTarget

	minified    bool
	showPreview bool
}

// New creates a new deactivated section.
func New(ctx context.Context, ctrl Controller, name string) *Section {
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionBrowse)
	list.SetActivateOnSingleClick(true)
	list.SetPlaceholder(gtk.NewLabel("No rooms yet..."))

	minify := newMinifyButton(true)

	inner := gtk.NewBox(gtk.OrientationVertical, 0)
	inner.Append(list)
	inner.Append(minify)

	rev := gtk.NewRevealer()
	rev.SetRevealChild(true)
	rev.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	rev.SetChild(inner)

	btn := newRevealButton(rev, name)
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
			{"Sort by", "roomsection.change-sort", s.sortByBox()},
			{"Appearance", "---", nil},
			{"Show Preview", "roomsection.show-preview", s.showPreviewBox()},
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
		s.current = matrix.RoomID(row.Name())
		ctrl.OpenRoom(s.current)
	})

	client := gotktrix.FromContext(ctx)
	s.comparer = roomsort.NewComparer(client.Offline(), roomsort.SortActivity)

	s.listBox.SetSortFunc(func(i, j *gtk.ListBoxRow) int {
		return s.comparer.Compare(
			matrix.RoomID(i.Name()),
			matrix.RoomID(j.Name()),
		)
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
	s.setSectionDropMode()

	return &s
}

// BeginReorderMode implements selfbar's.
func (s *Section) BeginReorderMode() {
	s.setRoomDropMode()
}

// EndReorderMode stops reordering mode.
func (s *Section) EndReorderMode() {
	s.setSectionDropMode()
}

func (s *Section) setRoomDropMode() {
	if s.sectionDrop != nil {
		s.listBox.RemoveController(s.sectionDrop)
	}

	if s.reordering == nil {
		s.reordering = gtkutil.NewListDropTarget(s.listBox, glib.TypeString, gdk.ActionMove)
		s.reordering.Connect("drop", func(_ *gtk.DropTarget, v *glib.Value, x, y float64) bool {
			r, pos := gtkutil.RowAtY(s.listBox, y)
			if r == nil {
				log.Println("no row found at y")
				return false
			}

			srcID, ok := roomIDFromValue(v)
			if !ok {
				return false
			}

			dstID := matrix.RoomID(r.Name())

			dstRoom, ok := s.rooms[dstID]
			if !ok {
				log.Printf("unknown room dropped upon, ID %s", dstID)
				return false
			}

			return s.ctrl.MoveRoom(srcID, dstRoom, pos)
		})
	}

	s.listBox.AddController(s.reordering)
}

func (s *Section) setSectionDropMode() {
	if s.reordering != nil {
		s.listBox.RemoveController(s.reordering)
	}

	if s.sectionDrop == nil {
		s.sectionDrop = gtk.NewDropTarget(glib.TypeString, gdk.ActionMove)
		s.sectionDrop.Connect("drop", func(_ *gtk.DropTarget, v *glib.Value) bool {
			srcID, ok := roomIDFromValue(v)
			if !ok {
				return false
			}

			return s.ctrl.MoveRoomToSection(srcID, s)
		})
	}

	s.listBox.AddController(s.sectionDrop)
}

func roomIDFromValue(v *glib.Value) (matrix.RoomID, bool) {
	vstr, ok := v.GoValue().(string)
	if !ok {
		log.Printf("erroneous value not of type string, but %T", v.GoValue())
		return "", false
	}

	return matrix.RoomID(vstr), true
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
	case roomsort.SortName:
		radio.Current = 0
	case roomsort.SortActivity:
		radio.Current = 1
	}

	b := gtk.NewBox(gtk.OrientationVertical, 0)
	b.Append(header)
	b.Append(gtkutil.NewRadioButtons(radio, func(i int) {
		switch i {
		case 0:
			s.SetSortMode(roomsort.SortName)
		case 1:
			s.SetSortMode(roomsort.SortActivity)
		}
	}))

	return b
}

// OpenRoom calls the parent controller's.
func (s *Section) OpenRoom(id matrix.RoomID) { s.ctrl.OpenRoom(id) }

// OpenRoomInTab calls the parent controller's.
func (s *Section) OpenRoomInTab(id matrix.RoomID) { s.ctrl.OpenRoomInTab(id) }

// SetSortMode sets the sorting mode for each room.
func (s *Section) SetSortMode(mode roomsort.SortMode) {
	s.comparer = roomsort.NewComparer(gotktrix.FromContext(s.ctx).Offline(), mode)
	s.InvalidateSort()
}

// SortMode returns the section's current sort mode.
func (s *Section) SortMode() roomsort.SortMode {
	if s.comparer == nil {
		log.Panicln("SortMode called before SetParentList")
	}

	return s.comparer.Mode
}

// Unselect unselects the list of the current section. If the given current room
// ID is the same as what the list has, then nothing is done.
func (s *Section) Unselect(current matrix.RoomID) {
	if s.current != current {
		s.listBox.UnselectAll()
	}
}

// Select selects the room with the given ID. If an unknown ID is given, then
// the function panics.
func (s *Section) Select(id matrix.RoomID) {
	rm, ok := s.rooms[id]
	if !ok {
		log.Panicln("selecting unknown room", id)
	}

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

	if s.minified {
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
	s.UseRoomPositions(nil)
}

// UseRoomPositions invalidates the current section's sorting to use the given
// room positions.
func (s *Section) UseRoomPositions(pos roomsort.RoomPositions) {
	s.comparer.InvalidateRoomCache()
	s.comparer.InvalidatePositions(pos)
	s.listBox.InvalidateSort()
	s.Reminify()
}

// InvalidateFilter invalidates the filtler.
func (s *Section) InvalidateFilter() {
	s.listBox.InvalidateFilter()
	s.Reminify()
}

// Reminify restores the minified state.
func (s *Section) Reminify() {
	if !s.minified {
		return
	}

	s.expand()
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
}

func (s *Section) expand() {
	for r, hidden := range s.hidden {
		if hidden {
			s.listBox.Append(r.ListBoxRow)
			s.hidden[r] = false
		}
	}
}
