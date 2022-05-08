package section

import (
	"context"
	"sort"
	"strings"

	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

// MatrixSectionOrder is the order of default Matrix rooms.
var MatrixSectionOrder = map[matrix.TagName]int{
	matrix.TagFavourite:    0,
	DMSection:              1,
	RoomsSection:           2, // regular room
	matrix.TagLowPriority:  3,
	matrix.TagServerNotice: 4,
}

// Pseudo tag names.
const (
	InternalTagNamespace = "xyz.diamondb.gotktrix"

	DMSection    matrix.TagName = InternalTagNamespace + ".dm_section"
	RoomsSection matrix.TagName = InternalTagNamespace + ".rooms_section"
)

// TagIsIntern returns true if the given tag is a Matrix tag or a tag that
// belongs only to us.
func TagIsIntern(name matrix.TagName) bool {
	return name.HasNamespace(InternalTagNamespace)
}

// TagName returns the name of the given tag.
func TagName(ctx context.Context, name matrix.TagName) string {
	p := locale.FromContext(ctx)

	switch {
	case name.HasNamespace("m"):
		switch name {
		case matrix.TagFavourite: // Thanks, Matrix.
			return p.Sprint("Favorites")
		case matrix.TagLowPriority:
			return p.Sprint("Low Priority")
		case matrix.TagServerNotice:
			return p.Sprint("Server Notice")
		default:
			return strings.Title(strings.TrimPrefix("m.", string(name)))
		}
	case name.HasNamespace("u"):
		return strings.TrimPrefix(string(name), "u.")
	}

	switch name {
	case DMSection:
		return p.Sprint("People")
	case RoomsSection:
		return p.Sprint("Rooms")
	}

	return string(name)
}

// TagNamespace returns the tag's namespace.
func TagNamespace(name matrix.TagName) string {
	switch {
	case name.HasNamespace("m"):
		return "m"
	case name.HasNamespace("u"):
		return "u"
	default:
		return string(name)
	}
}

// TagEqNamespace returns true if n1 and n2 are in the same namespace.
func TagEqNamespace(n1, n2 matrix.TagName) bool {
	return TagNamespace(n1) == TagNamespace(n2)
}

// OrderedTag is a type that defines the tag order. It is used for sorting and
// prioritizing room tags.
type OrderedTag struct {
	Name  matrix.TagName
	Order float64 // 2 if nil
}

// RoomTags returns the tags of the given room sorted in a deterministic order.
func RoomTags(c *gotktrix.Client, id matrix.RoomID) []OrderedTag {
	e, err := c.RoomEvent(id, event.TypeTag)
	if err != nil {
		return defaultRoomTag(c, id)
	}

	ev := e.(*event.TagEvent)
	if len(ev.Tags) == 0 {
		return defaultRoomTag(c, id)
	}

	return NewOrderedTags(ev.Tags)
}

// NewOrderedTags creates a new list of
func NewOrderedTags(tags map[matrix.TagName]matrix.Tag) []OrderedTag {
	ordtags := make([]OrderedTag, 0, len(tags))

	for name, tag := range tags {
		order := 2.0
		if tag.Order != nil {
			order = *tag.Order
		}

		ordtags = append(ordtags, OrderedTag{name, order})
	}

	SortTags(ordtags)
	return ordtags
}

// SortTags sorts the given list of ordered tags in a deterministic order.
func SortTags(tags []OrderedTag) {
	if len(tags) < 2 {
		return
	}

	// Sort the tags in ascending order. Rooms that are supposed to appear first
	// will appear first.
	sort.Slice(tags, func(i, j int) bool {
		// Prioritize user tags.
		if !TagEqNamespace(tags[i].Name, tags[j].Name) {
			if tags[i].Name.HasNamespace("u") {
				return true
			}
			if tags[j].Name.HasNamespace("u") {
				return false
			}
		}

		if tags[i].Order != tags[j].Order {
			// Tag the room so that it will be in the section with the topmost
			// order.
			return tags[i].Order < tags[j].Order
		}

		// Fallback to tag name.
		return sortutil.LessFold(string(tags[i].Name), string(tags[j].Name))
	})
}

// RoomTag queries the client and returns the tag that the room with the given
// ID is in. It tries its best to be deterministic. If the room should be in the
// default room section, then an empty string is returned.
func RoomTag(c *gotktrix.Client, id matrix.RoomID) matrix.TagName {
	tags := RoomTags(c, id)
	return tags[0].Name
}

func defaultRoomTag(c *gotktrix.Client, id matrix.RoomID) []OrderedTag {
	if c.IsDirect(id) {
		return []OrderedTag{{Name: DMSection, Order: 2}}
	} else {
		return []OrderedTag{{Name: RoomsSection, Order: 2}}
	}
}
