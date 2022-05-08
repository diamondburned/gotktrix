package section

import (
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
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
	roomTags map[matrix.RoomID]float64
	roomData map[matrix.RoomID]interface{} // name or content
}

// NewComparer creates a new comparer.
func NewComparer(client *gotktrix.Client, mode SortMode, tag matrix.TagName) *Comparer {
	return &Comparer{
		Tag:      tag,
		Mode:     mode,
		client:   client,
		roomTags: map[matrix.RoomID]float64{},
		roomData: map[matrix.RoomID]interface{}{},
	}
}

// InvalidateRoomCache invalidates the room name/timestamp cache. This is
// automatically called when Sort is called.
func (c *Comparer) InvalidateRoomCache() {
	c.roomTags = make(map[matrix.RoomID]float64, len(c.roomTags))
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

	if ipos != jpos {
		if ipos < jpos {
			return -1
		}
		if ipos > jpos {
			return 1
		}
	}

	return c.compare(iID, jID)
}

// roomOrder gets the order number of the given room within the range [0, 1]. If
// the room does not have the order number, then 2 is returned.
func (c *Comparer) roomOrder(id matrix.RoomID) float64 {
	o, ok := c.roomTags[id]
	if ok {
		return o
	}

	e, err := c.client.RoomEvent(id, event.TypeTag)
	if err != nil {
		// Prevent cache misses on erroneous rooms.
		c.roomTags[id] = 2
		return 2
	}

	t, ok := e.(*event.TagEvent).Tags[c.Tag]
	if ok && t.Order != nil {
		c.roomTags[id] = *t.Order
		return *t.Order
	} else {
		c.roomTags[id] = 2
		return 2
	}
}

func (c *Comparer) roomTimestamp(id matrix.RoomID) int64 {
	if v, ok := c.roomData[id]; ok {
		i, _ := v.(int64)
		return i
	}

	var ev event.Type
	if messageOnly.Value() {
		ev = event.TypeRoomMessage
	}

	found, _ := c.client.State.LatestInTimeline(id, ev)
	if found == nil {
		c.roomData[id] = nil
		return 0
	}

	ts := int64(found.RoomInfo().OriginServerTime)
	c.roomData[id] = ts

	return ts
}

func (c *Comparer) roomName(id matrix.RoomID) string {
	if v, ok := c.roomData[id]; ok {
		s, _ := v.(string)
		return s
	}

	name, err := c.client.RoomName(id)
	if err != nil {
		// Set something just so we don't repetitively hit the API.
		c.roomData[id] = nil
		return ""
	}

	c.roomData[id] = name
	return name
}

func (c *Comparer) compare(iID, jID matrix.RoomID) int {
	switch c.Mode {
	case SortActivity:
		its := c.roomTimestamp(iID)
		jts := c.roomTimestamp(jID)

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
		iname := c.roomName(iID)
		jname := c.roomName(jID)

		if iname == "" {
			return 1 // put to last
		}
		if jname == "" {
			return -1 // put to iast
		}

		return sortutil.CmpFold(iname, jname)
	}

	return 0
}
