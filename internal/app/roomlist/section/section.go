package section

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
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

const nMinified = 8

// DMSection is the special pseudo-tag name for the DM section.
const DMSection matrix.TagName = "xyz.diamondb.gotktrix.dm_section"

// TagName returns the name of the given tag.
func TagName(name matrix.TagName) string {
	switch {
	case name.Namespace("m"):
		switch name {
		case matrix.TagFavourites, "m.favourite": // Thanks, Matrix.
			return "Favorites"
		case matrix.TagLowPriority:
			return "Low Priority"
		case matrix.TagServerNotice:
			return "Server Notice"
		}
	case name.Namespace("u"):
		return strings.TrimPrefix(string(name), "u.")
	case name == DMSection:
		return "People"
	case name == "":
		return "Rooms"
	}

	return string(name)
}

// TagNamespace returns the tag's namespace.
func TagNamespace(name matrix.TagName) string {
	switch {
	case name.Namespace("m"):
		return "m"
	case name.Namespace("u"):
		return "u"
	default:
		return string(name)
	}
}

// TagEqNamespace returns true if n1 and n2 are in the same namespace.
func TagEqNamespace(n1, n2 matrix.TagName) bool {
	return TagNamespace(n1) == TagNamespace(n2)
}

// RoomTag queries the client and returns the tag that the room with the given
// ID is in. It tries its best to be deterministic. If the room should be in the
// default room section, then an empty string is returned.
func RoomTag(c *gotktrix.Client, id matrix.RoomID) matrix.TagName {
	e, err := c.RoomEvent(id, event.TypeTag)
	if err != nil {
		return defaultRoomTag(c, id)
	}

	ev := e.(event.TagEvent)
	if len(ev.Tags) == 0 {
		return defaultRoomTag(c, id)
	}

	type roomTag struct {
		Name  matrix.TagName
		Order float64 // 2 if nil
	}

	// TODO: priorize u. tags

	tags := make([]roomTag, 0, len(ev.Tags))
	for name, tag := range ev.Tags {
		order := 2.0
		if tag.Order != nil {
			order = *tag.Order
		}

		tags = append(tags, roomTag{name, order})
	}

	if len(tags) == 1 {
		return tags[0].Name
	}

	// Sort the tags in ascending order. Rooms that are supposed to appear first
	// will appear first.
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Order != tags[j].Order {
			// Tag the room so that it will be in the section with the topmost
			// order.
			return tags[i].Order < tags[j].Order
		}

		// Prioritize user tags.
		if !TagEqNamespace(tags[i].Name, tags[j].Name) {
			if tags[i].Name.Namespace("u") {
				return true
			}
			if tags[j].Name.Namespace("u") {
				return false
			}
		}

		// Fallback to tag name.
		return sortutil.LessFold(string(tags[i].Name), string(tags[j].Name))
	})

	return tags[0].Name
}

func defaultRoomTag(c *gotktrix.Client, id matrix.RoomID) matrix.TagName {
	if c.IsDirect(id) {
		return DMSection
	} else {
		return ""
	}
}

// Controller describes the parent widget that Section controls.
type Controller interface {
	OpenRoom(matrix.RoomID)
	OpenRoomInTab(matrix.RoomID)

	// Searching returns the string being searched.
	Searching() string

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

	comparer Comparer
	current  matrix.RoomID

	minified    bool
	showPreview bool
}

// New creates a new deactivated section.
func New(ctx context.Context, ctrl Controller, tag matrix.TagName) *Section {
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionBrowse)
	list.SetActivateOnSingleClick(true)
	list.SetPlaceholder(gtk.NewLabel("No rooms yet..."))

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
			{"Sort By", "roomsection.change-sort", s.sortByBox()},
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

// SetSortMode sets the sorting mode for each room.
func (s *Section) SetSortMode(mode SortMode) {
	s.comparer = *NewComparer(gotktrix.FromContext(s.ctx).Offline(), mode, s.comparer.Tag)
	s.InvalidateSort()
}

// SortMode returns the section's current sort mode.
func (s *Section) SortMode() SortMode {
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
	if !s.minified || len(s.rooms) < nMinified {
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
