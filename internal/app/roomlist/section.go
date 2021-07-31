package roomlist

import (
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/roomsort"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

// Section is a room section, such as People or Favorites.
type Section struct {
	*gtk.Box
	List *gtk.ListBox

	current matrix.RoomID
	parent  *List
}

var expandCSS = cssutil.Applier("roomlist-expand", `
	.roomlist-expand {
		padding: 2px;
		border-radius: 0;
	}
	.roomlist-expand image {
		min-width: 32px;
		margin:  2px 4px;
		padding: 0;
	}
`)

var expandLabelAttrs = markuputil.Attrs(
	pango.NewAttrScale(0.9),
	pango.NewAttrForegroundAlpha(65535*85/100), // 85%
	pango.NewAttrWeight(pango.WeightBook),
)

func newRevealButton(rev *gtk.Revealer, name string) *gtk.ToggleButton {
	arrow := gtk.NewImageFromIconName(revealIconName(rev.RevealChild()))
	arrow.SetPixelSize(16)

	label := gtk.NewLabel(name)
	label.SetAttributes(expandLabelAttrs)
	label.SetHExpand(true)
	label.SetXAlign(0)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(arrow)
	box.Append(label)

	button := gtk.NewToggleButton()
	button.SetActive(rev.RevealChild())
	button.SetChild(box)
	button.Connect("toggled", func(button *gtk.ToggleButton) {
		reveal := button.Active()
		rev.SetRevealChild(reveal)
		arrow.SetFromIconName(revealIconName(reveal))
	})
	expandCSS(button)

	return button
}

func revealIconName(rev bool) string {
	if rev {
		return "go-down"
	}
	return "go-next"
}

// NewSection creates a new deactivated section.
func NewSection(name string) *Section {
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

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetSizeRequest(200, -1)
	box.Append(btn)
	box.Append(rev)

	section := Section{
		Box:  box,
		List: list,
	}

	return &section
}

// SetParentList sets the section's parent list and activates it.
func (s *Section) SetParentList(parent *List) {
	s.parent = parent

	s.List.Connect("row-activated", func(list *gtk.ListBox, row *gtk.ListBoxRow) {
		s.current = matrix.RoomID(row.Name())
		parent.setRoom(s.current)
	})

	comparer := roomsort.NewComparer(
		parent.client.Offline(),
		roomsort.SortAlphabetically,
	)

	s.List.SetSortFunc(func(i, j *gtk.ListBoxRow) int {
		return comparer.Compare(
			matrix.RoomID(i.Name()),
			matrix.RoomID(j.Name()),
		)
	})

	s.List.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
		if parent.search == "" {
			return true
		}

		rm, ok := parent.rooms[matrix.RoomID(row.Name())]
		if !ok {
			return false
		}

		return strings.Contains(rm.name, parent.search)
	})
}

// Unselect unselects the list of the current section. If the given current room
// ID is the same as what the list has, then nothing is done.
func (s *Section) Unselect(current matrix.RoomID) {
	if s.current != current {
		s.List.UnselectAll()
	}
}
