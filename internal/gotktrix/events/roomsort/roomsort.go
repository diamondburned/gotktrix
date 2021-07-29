// Package roomsort provides an implementation of the rsort room sorting protocol.
package roomsort

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/state"
	"github.com/pkg/errors"
)

func init() {
	event.Register(RoomPositionEventType, parseRoomPositionEvent)
}

const (
	RoomPositionEventType event.Type = "xyz.diamondb.gotktrix.room_position"
)

// Anchor describes the relationship of the position ID of a room.
type Anchor string

const (
	AnchorAbove  Anchor = "above"  // needs ID
	AnchorBelow  Anchor = "below"  // needs ID
	AnchorTop    Anchor = "top"    // undefined if multiple
	AnchorBottom Anchor = "bottom" // undefined if multiple
)

// RoomPositionEvent describes the xyz.diamondb.gotktrix.room_position event.
//
// A RoomPositionEvent turns each room into a node in a linked list. If the
// room's position is anchored top or bottom (or not at all), then it is the
// head of the linked list. Rooms that have anchors above or below another room
// can be considered another node in that linked list.
//
// If a room has the same exact anchor parameters, for example, when two rooms
// have a top anchor, or when two rooms have an anchor above the same room, then
// the position is undefined. Usually, clients should fallback to sorting by
// name or activity, whichever the user's preference may be.
type RoomPositionEvent struct {
	Positions RoomPositions `json:"positions"`
}

func parseRoomPositionEvent(raw event.RawEvent) (event.Event, error) {
	var ev RoomPositionEvent
	if raw.Type != ev.Type() {
		return nil, fmt.Errorf("unexpected event type %q", raw.Type)
	}

	if err := json.Unmarshal(raw.Content, &ev); err != nil {
		return nil, errors.Wrap(err, "faileld to unmarshal RoomPositionEvent")
	}

	return ev, nil
}

// Type implements event.Type.
func (ev RoomPositionEvent) Type() event.Type { return RoomPositionEventType }

// RoomPositions maps a room ID to its position.
type RoomPositions map[matrix.RoomID]RoomPosition

// RoomPosition describes the position of a room, which is determined by the
// anchor of the position. If the anchor is above or below, then the relative ID
// is used to determine where exactly it should be.
type RoomPosition struct {
	Anchor Anchor        `json:"anchor"`
	RelID  matrix.RoomID `json:"rel_id,omitempty"`
}

// SortMode describes the possible ways to sort a room.
type SortMode uint8

const (
	SortAlphabetically SortMode = iota
	SortActivity
)

// Sorter is a sorter interface that implements sort.Interface. It sorts rooms.
type Sorter struct {
	// SwapFn is an additional function that the user could use to synchronize
	// sorting with another data structure, such as a ListBox.
	SwapFn func(i, j int)

	Rooms []matrix.RoomID
	Mode  SortMode

	positions   RoomPositions
	roomDataMap map[matrix.RoomID]interface{} // name or content
	client      *gotktrix.Client
}

// SortedRooms returns the list of rooms sorted.
func SortedRooms(client *gotktrix.Client, mode SortMode) ([]matrix.RoomID, error) {
	rooms, err := client.Rooms()
	if err != nil {
		return nil, err
	}

	Sort(client, rooms, mode)
	return rooms, nil
}

// Sort sorts a room using a Sorter. It's a convenient function.
func Sort(client *gotktrix.Client, rooms []matrix.RoomID, mode SortMode) {
	sorter := NewSorter(client, rooms, mode)
	sorter.Sort()
}

// NewSorter creates a new sorter.
func NewSorter(client *gotktrix.Client, rooms []matrix.RoomID, mode SortMode) *Sorter {
	return &Sorter{
		client:      client.WithContext(state.Cancelled()),
		Rooms:       rooms,
		Mode:        mode,
		roomDataMap: map[matrix.RoomID]interface{}{},
	}
}

// Add adds the given room IDs and resort the whole list.
func (sorter *Sorter) Add(ids ...matrix.RoomID) {
	sorter.Rooms = append(sorter.Rooms, ids...)
	sorter.Sort()
}

// Sort sorts the sorter.
func (sorter *Sorter) Sort() {
	sorter.InvalidateRoomCache()
	sort.Sort(sorter)
}

// InvalidatePositions invalidates the room_positions event cached inside the
// sorter.
func (sorter *Sorter) InvalidatePositions() error {
	ev, err := sorter.client.UserEvent(RoomPositionEventType)
	if err != nil {
		return err
	}

	pos, _ := ev.(RoomPositionEvent)
	sorter.positions = pos.Positions

	return nil
}

// InvalidateRoomCache invalidates the room name/timestamp cache. This is
// automatically called when Sort is called.
func (sorter *Sorter) InvalidateRoomCache() {
	if len(sorter.roomDataMap) == 0 {
		// Already empty.
		return
	}

	sorter.roomDataMap = make(map[matrix.RoomID]interface{}, len(sorter.roomDataMap))
}

// Len returns the length of sorter.Rooms.
func (sorter *Sorter) Len() int { return len(sorter.Rooms) }

// Swap swaps the entries inside Rooms.
func (sorter *Sorter) Swap(i, j int) {
	sorter.Rooms[i], sorter.Rooms[j] = sorter.Rooms[j], sorter.Rooms[i]

	if sorter.SwapFn != nil {
		sorter.SwapFn(i, j)
	}
}

// Less returns true if the room indexed [i] should be before [j].
func (sorter *Sorter) Less(i, j int) bool {
	iID := sorter.Rooms[i]
	jID := sorter.Rooms[j]

	switch ipos := sorter.positions[iID]; ipos.Anchor {
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
		if jID == sorter.Rooms[0] {
			// j is the first room, so place i before it.
			return true
		}
	case AnchorBottom:
		if jID == sorter.Rooms[len(sorter.Rooms)-1] {
			// j is the last room, so place i after it.
			return false
		}
	}

	return sorter.less(iID, jID)
}

func (sorter *Sorter) roomTimestamp(id matrix.RoomID) (int64, bool) {
	if v, ok := sorter.roomDataMap[id]; ok {
		if v == nil {
			return 0, false
		}

		return v.(int64), true
	}

	events, err := sorter.client.RoomTimeline(id)
	if err != nil || len(events) == 0 {
		// Set something just so we don't repetitively hit the API.
		sorter.roomDataMap[id] = nil
		return 0, false
	}

	ts := int64(events[0].OriginServerTime())
	sorter.roomDataMap[id] = ts

	return ts, true
}

func (sorter *Sorter) roomName(id matrix.RoomID) (string, bool) {
	if v, ok := sorter.roomDataMap[id]; ok {
		if v == nil {
			return "", false
		}

		return v.(string), true
	}

	name, err := sorter.client.RoomName(id)
	if err != nil {
		// Set something just so we don't repetitively hit the API.
		sorter.roomDataMap[id] = nil
		return "", false
	}

	sorter.roomDataMap[id] = name
	return name, true
}

func (sorter *Sorter) less(iID, jID matrix.RoomID) bool {
	switch sorter.Mode {
	case SortActivity:
		its, iok := sorter.roomTimestamp(iID)
		jts, jok := sorter.roomTimestamp(jID)

		if !iok {
			return false
		}
		if !jok {
			return true
		}

		return its < jts

	case SortAlphabetically:
		iname, iok := sorter.roomName(iID)
		jname, jok := sorter.roomName(jID)

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
