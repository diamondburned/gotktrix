// Package roomsort provides an implementation of the rsort room sorting protocol.
package roomsort

import (
	"encoding/json"
	"fmt"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
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
