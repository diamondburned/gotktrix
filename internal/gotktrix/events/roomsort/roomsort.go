// Package roomsort provides an implementation of the rsort room sorting protocol.
package roomsort

import (
	"encoding/json"
	"fmt"
	"sort"

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

// Sanitize ensures that the room position map is valid, in that there is no
// recursion and no duplicate top/bottom anchors. This should not be used as a
// reliable way to sort rooms, as the user might not be aware that some of their
// settings are removed.
func (p RoomPositions) Sanitize() {
	keys := make([]string, 0, len(p))
	rels := make(map[string]struct{}, len(p))

	for k, pos := range p {
		keys = append(keys, string(k))
		rels[string(pos.RelID)] = struct{}{}
	}

	sort.Strings(keys)

	var hasTop bool
	var hasBottom bool

	// Scan backwards and remove the last few recursive definitions.
	for i := len(p) - 1; i >= 0; i-- {
		roomID := matrix.RoomID(keys[i])
		if _, recursive := rels[keys[i]]; recursive {
			delete(p, roomID)
			continue
		}

		if pos, ok := p[roomID]; ok {
			switch pos.Anchor {
			case AnchorTop:
				if hasTop {
					delete(p, roomID)
				} else {
					hasTop = true
				}
			case AnchorBottom:
				if hasBottom {
					delete(p, roomID)
				} else {
					hasBottom = true
				}
			}
		}
	}
}

// HasRelative returns true if any of the room positions inside p already has a
// relative ID set to the given room ID. It is used to check against recursions.
func (p RoomPositions) HasRelative(roomID matrix.RoomID) bool {
	return p.find(func(r RoomPosition) bool { return r.RelID == roomID }) != ""
}

// Set sets the position override into p.
//
// If there is already a room anchored to the top or bottom, then it steals that
// room's position and give it to the room with the given ID. The old room will
// be anchored before or after the new room.
//
// If the given anchor is not top or bottom, then the position is appended
// normally, unless it is recursive, in which the old definition is deleted. If
// there is already a room with the same anchor and relative ID as the current
// one, then it is
func (p RoomPositions) Set(roomID matrix.RoomID, pos RoomPosition) {
	switch pos.Anchor {
	case AnchorAbove, AnchorBelow:
		p.addRelative(roomID, pos)
	case AnchorTop, AnchorBottom:
		p.addAnchored(roomID, pos.Anchor)
	}
}

func (p RoomPositions) addRelative(roomID matrix.RoomID, pos RoomPosition) {
	// Ensure that there is no recursive definition for this room.
	existingID := p.find(func(r RoomPosition) bool { return r.RelID == roomID })
	if existingID != "" {
		delete(p, existingID)
	}

	// Ensure that a duplicate definition doesn't exist.
	p.moveAnchor(roomID, pos)

	p[roomID] = pos
}

func (p RoomPositions) addAnchored(roomID matrix.RoomID, anchor Anchor) {
	// Ensure that only one anchor of the same kind exists.
	existingID := p.find(func(r RoomPosition) bool { return r.Anchor == anchor })
	if existingID != "" {
		existing := p[existingID]

		switch anchor {
		case AnchorTop:
			existing.Anchor = AnchorBelow
			existing.RelID = roomID
		case AnchorBottom:
			existing.Anchor = AnchorAbove
			existing.RelID = roomID
		}

		p[existingID] = existing
		// Ensure that there exists no override that already links to the room
		// we just got, especially because we changed the existing room's
		// anchor.
		p.moveAnchor(roomID, RoomPosition{Anchor: anchor})
	}

	p[roomID] = RoomPosition{
		Anchor: anchor,
	}
}

func (p RoomPositions) moveAnchor(roomID matrix.RoomID, pos RoomPosition) {
	existingID := p.find(func(r RoomPosition) bool { return r == pos })
	if existingID == "" {
		return
	}

	existing := p[existingID]
	existing.RelID = roomID
	// Same anchor.

	// Recursively traverse the tree to ensure that our changes did not cause a
	// duplicate.
	// TODO: get rid of this recursion.
	p.moveAnchor(existingID, existing)
}

func (p RoomPositions) find(f func(r RoomPosition) bool) matrix.RoomID {
	for r, pos := range p {
		if f(pos) {
			return r
		}
	}
	return ""
}

// RoomPosition describes the position of a room, which is determined by the
// anchor of the position. If the anchor is above or below, then the relative ID
// is used to determine where exactly it should be.
type RoomPosition struct {
	Anchor Anchor        `json:"anchor"`
	RelID  matrix.RoomID `json:"rel_id,omitempty"`
}
