package section

import (
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/sortutil"
)

// SortMode describes the possible ways to sort a room.
type SortMode uint8

const (
	SortName SortMode = iota // A-Z
	SortActivity
)

// Comparer partially implements sort.Interface: it provides a Less function
// that Sorter can easily build upon, but exposed for other uses.
type Comparer struct {
	// Tag is the tag that this comparer should use. If this is an empty string,
	// then Comparer will ignore room tags completely.
	Tag matrix.TagName
	// Mode is the sorting mode that this comparer should do.
	Mode SortMode

	client   *gotktrix.Client // offline
	roomTags map[matrix.RoomID]event.TagEvent
	roomData map[matrix.RoomID]interface{} // name or content
}

// NewComparer creates a new comparer.
func NewComparer(client *gotktrix.Client, mode SortMode, tag matrix.TagName) *Comparer {
	return &Comparer{
		Tag:      tag,
		Mode:     mode,
		client:   client,
		roomTags: map[matrix.RoomID]event.TagEvent{},
		roomData: map[matrix.RoomID]interface{}{},
	}
}

// InvalidateRoomCache invalidates the room name/timestamp cache. This is
// automatically called when Sort is called.
func (c *Comparer) InvalidateRoomCache() {
	c.roomTags = make(map[matrix.RoomID]event.TagEvent, len(c.roomTags))
	c.roomData = make(map[matrix.RoomID]interface{}, len(c.roomData))
}

// Less returns true if the room with iID should be before the one with jID.
func (c *Comparer) Less(iID, jID matrix.RoomID) bool {
	return c.Compare(iID, jID) == -1
}

// Compare behaves similarly to strings.Compare. Most users should use Less
// instead of Compare; this method only exists to satisfy bad C APIs.
//
// As the API is similar to strings.Compare, 0 is returned if the position is
// equal; -1 is returned if iID's position is less than (above/before) jID, and
// 1 is returned if iID's position is more than (below/after) jID.
func (c *Comparer) Compare(iID, jID matrix.RoomID) int {
	if iID == jID {
		return 0
	}

	ipos := c.roomOrder(iID)
	jpos := c.roomOrder(jID)

	if ipos == -1 && jpos == -1 {
		return c.compare(iID, jID)
	}

	if ipos < jpos {
		return -1
	}
	if ipos == jpos {
		return 0
	}
	return 1
}

// roomOrder gets the order number of the given room within the range [0, 1]. If
// the room does not have the order number, then 2 is returned.
func (c *Comparer) roomOrder(id matrix.RoomID) float64 {
	tag, ok := c.roomTags[id]
	if !ok {
		e, err := c.client.RoomEvent(id, event.TypeTag)
		if err != nil {
			// Prevent cache misses on erroneous rooms.
			c.roomTags[id] = event.TagEvent{}
			return 2
		}

		tag := e.(event.TagEvent)
		c.roomTags[id] = tag
	}

	if o, ok := tag.Tags[c.Tag]; ok && o.Order != nil {
		return *o.Order
	}

	return -1
}

func (c *Comparer) roomTimestamp(id matrix.RoomID) int64 {
	if v, ok := c.roomData[id]; ok {
		i, _ := v.(int64)
		return i
	}

	events, err := c.client.RoomTimeline(id)
	if err != nil || len(events) == 0 {
		// Set something just so we don't repetitively hit the API.
		c.roomData[id] = nil
		return 0
	}

	ts := int64(events[len(events)-1].OriginServerTime())
	c.roomData[id] = ts

	return ts
}

func (comparer *Comparer) roomName(id matrix.RoomID) string {
	if v, ok := comparer.roomData[id]; ok {
		s, _ := v.(string)
		return s
	}

	name, err := comparer.client.RoomName(id)
	if err != nil {
		// Set something just so we don't repetitively hit the API.
		comparer.roomData[id] = nil
		return ""
	}

	comparer.roomData[id] = name
	return name
}

func (comparer *Comparer) compare(iID, jID matrix.RoomID) int {
	switch comparer.Mode {
	case SortActivity:
		its := comparer.roomTimestamp(iID)
		jts := comparer.roomTimestamp(jID)

		if its == 0 {
			return 1 // put to last
		}
		if jts == 0 {
			return -1 // put to last
		}

		if its == jts {
			return 0
		}
		if its < jts {
			return 1
		}
		return -1

	case SortName:
		iname := comparer.roomName(iID)
		jname := comparer.roomName(jID)

		if iname == "" {
			return 1 // put to last
		}
		if jname == "" {
			return -1 // put to iast
		}

		return sortutil.StrcmpFold(iname, jname)
	}

	return 0
}
