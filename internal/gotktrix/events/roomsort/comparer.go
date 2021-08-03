package roomsort

import (
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/sortutil"
)

// PartialRoomLister is needed by Comparer to get the first and last room for
// anchoring. It should directly reflect the result of the list structure that
// Comparer is acting upon.
type PartialRoomLister interface {
	FirstID() matrix.RoomID
	LastID() matrix.RoomID
}

// Comparer partially implements sort.Interface: it provides a Less function
// that Sorter can easily build upon, but exposed for other uses.
type Comparer struct {
	Mode SortMode
	List PartialRoomLister // optional

	positions   RoomPositions
	roomDataMap map[matrix.RoomID]interface{} // name or content
	client      *gotktrix.Client              // offline
}

// NewComparer creates a new comparer.
func NewComparer(client *gotktrix.Client, mode SortMode) *Comparer {
	return &Comparer{
		client:      client,
		Mode:        mode,
		roomDataMap: map[matrix.RoomID]interface{}{},
	}
}

// InvalidatePositions invalidates the room_positions event cached inside the
// sorter.
func (comparer *Comparer) InvalidatePositions() error {
	ev, err := comparer.client.UserEvent(RoomPositionEventType)
	if err != nil {
		return err
	}

	pos, _ := ev.(RoomPositionEvent)
	comparer.positions = pos.Positions

	return nil
}

// InvalidateRoomCache invalidates the room name/timestamp cache. This is
// automatically called when Sort is called.
func (comparer *Comparer) InvalidateRoomCache() {
	if len(comparer.roomDataMap) == 0 {
		// Already empty.
		return
	}

	comparer.roomDataMap = make(map[matrix.RoomID]interface{}, len(comparer.roomDataMap))
}

// Less returns true if the room with iID should be before the one with jID.
func (comparer *Comparer) Less(iID, jID matrix.RoomID) bool {
	return comparer.Compare(iID, jID) == -1
}

// Compare behaves similarly to strings.Compare. Most users should use Less
// instead of Compare; this method only exists to satisfy bad C APIs.
//
// As the API is similar to strings.Compare, 0 is returned if the position is
// equal; -1 is returned if iID's position is less than (above/before) jID, and
// 1 is returned if iID's position is more than (below/after) jID.
func (comparer *Comparer) Compare(iID, jID matrix.RoomID) int {
	if iID == jID {
		return 0
	}

	switch ipos := comparer.positions[iID]; ipos.Anchor {
	case AnchorAbove:
		if ipos.RelID == jID {
			return -1
		}
	case AnchorBelow:
		if ipos.RelID == jID {
			return 1
		}
	case AnchorTop:
		return -1 // always before

		// if comparer.List != nil && jID == comparer.List.FirstID() {
		// 	return -1
		// }
	case AnchorBottom:
		return 1 // always after

		// if comparer.List != nil && jID == comparer.List.LastID() {
		// 	return false
		// }
	}

	return comparer.compare(iID, jID)
}

func (comparer *Comparer) roomTimestamp(id matrix.RoomID) int64 {
	if v, ok := comparer.roomDataMap[id]; ok {
		i, _ := v.(int64)
		return i
	}

	events, err := comparer.client.RoomTimeline(id)
	if err != nil || len(events) == 0 {
		// Set something just so we don't repetitively hit the API.
		comparer.roomDataMap[id] = nil
		return 0
	}

	ts := int64(events[0].OriginServerTime())
	comparer.roomDataMap[id] = ts

	return ts
}

func (comparer *Comparer) roomName(id matrix.RoomID) string {
	if v, ok := comparer.roomDataMap[id]; ok {
		s, _ := v.(string)
		return s
	}

	name, err := comparer.client.RoomName(id)
	if err != nil {
		// Set something just so we don't repetitively hit the API.
		comparer.roomDataMap[id] = nil
		return ""
	}

	comparer.roomDataMap[id] = name
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

	case SortAlphabetically:
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
