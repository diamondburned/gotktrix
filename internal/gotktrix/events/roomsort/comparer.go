package roomsort

import (
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
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
		client:      client.Offline(),
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
	switch ipos := comparer.positions[iID]; ipos.Anchor {
	case AnchorAbove:
		if ipos.RelID == jID {
			// i is above (before) j.
			return true
		}
	case AnchorBelow:
		if ipos.RelID == jID {
			// i is below (after) j.
			return false
		}
	case AnchorTop:
		if comparer.List != nil && jID == comparer.List.FirstID() {
			// j is the first room, so place i before it.
			return true
		}
	case AnchorBottom:
		if comparer.List != nil && jID == comparer.List.LastID() {
			// j is the last room, so place i after it.
			return false
		}
	}

	return comparer.less(iID, jID)
}

func (comparer *Comparer) roomTimestamp(id matrix.RoomID) (int64, bool) {
	if v, ok := comparer.roomDataMap[id]; ok {
		if v == nil {
			return 0, false
		}

		return v.(int64), true
	}

	events, err := comparer.client.RoomTimeline(id)
	if err != nil || len(events) == 0 {
		// Set something just so we don't repetitively hit the API.
		comparer.roomDataMap[id] = nil
		return 0, false
	}

	ts := int64(events[0].OriginServerTime())
	comparer.roomDataMap[id] = ts

	return ts, true
}

func (comparer *Comparer) roomName(id matrix.RoomID) (string, bool) {
	if v, ok := comparer.roomDataMap[id]; ok {
		if v == nil {
			return "", false
		}

		return v.(string), true
	}

	name, err := comparer.client.RoomName(id)
	if err != nil {
		// Set something just so we don't repetitively hit the API.
		comparer.roomDataMap[id] = nil
		return "", false
	}

	comparer.roomDataMap[id] = name
	return name, true
}

func (comparer *Comparer) less(iID, jID matrix.RoomID) bool {
	switch comparer.Mode {
	case SortActivity:
		its, iok := comparer.roomTimestamp(iID)
		jts, jok := comparer.roomTimestamp(jID)

		if !iok {
			return false
		}
		if !jok {
			return true
		}

		return its < jts

	case SortAlphabetically:
		iname, iok := comparer.roomName(iID)
		jname, jok := comparer.roomName(jID)

		if !iok {
			return false
		}
		if !jok {
			return true
		}

		return iname < jname
	}

	return false
}
