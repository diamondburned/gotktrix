package section

import (
	"fmt"
	"log"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/roomsort"
)

const nMinified = 8

// ParentList describes the list containing the section.
type ParentList interface {
	Client() *gotktrix.Client
	OpenRoom(matrix.RoomID)
	OpenRoomInTab(matrix.RoomID)

	// Searching returns the string being searched.
	Searching() string
}

// Section is a room section, such as People or Favorites.
type Section struct {
	ParentList

	*gtk.Box
	listBox *gtk.ListBox
	minify  *minifyButton

	rooms  map[matrix.RoomID]*room.Room
	hidden map[*room.Room]bool

	current matrix.RoomID

	minified bool
}

// New creates a new deactivated section.
func New(name string) *Section {
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionBrowse)
	list.SetActivateOnSingleClick(true)
	list.SetPlaceholder(gtk.NewLabel("No rooms yet..."))

	rev := gtk.NewRevealer()
	rev.SetRevealChild(true)
	rev.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	rev.SetChild(list)

	btn := newRevealButton(rev, name)
	btn.SetHasFrame(false)

	minify := newMinifyButton(true)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetSizeRequest(200, -1)
	box.Append(btn)
	box.Append(rev)
	box.Append(minify)
	box.SetSensitive(false)

	sect := Section{
		Box:     box,
		minify:  minify,
		rooms:   make(map[matrix.RoomID]*room.Room),
		hidden:  make(map[*room.Room]bool),
		listBox: list,
	}

	minify.OnToggled(func(minify bool) string {
		if !minify {
			sect.Expand()
			return "Show less"
		}

		sect.Minimize()
		return fmt.Sprintf("Show %d more", sect.NHidden())
	})

	return &sect
}

// SetParentList sets the section's parent list and activates it.
func (s *Section) SetParentList(parent ParentList) {
	s.ParentList = parent
	s.Box.SetSensitive(true)

	s.listBox.Connect("row-activated", func(list *gtk.ListBox, row *gtk.ListBoxRow) {
		s.current = matrix.RoomID(row.Name())
		parent.OpenRoom(s.current)
	})

	comparer := roomsort.NewComparer(
		parent.Client().Offline(),
		// TODO enumify
		roomsort.SortActivity,
	)

	s.listBox.SetSortFunc(func(i, j *gtk.ListBoxRow) int {
		return comparer.Compare(
			matrix.RoomID(i.Name()),
			matrix.RoomID(j.Name()),
		)
	})

	s.listBox.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
		searching := parent.Searching()
		if searching == "" {
			return true
		}

		rm, ok := s.rooms[matrix.RoomID(row.Name())]
		if !ok {
			return false
		}

		return strings.Contains(rm.Name, searching)
	})
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
