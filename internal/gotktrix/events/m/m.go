// Package m provides Matrix-namespace events.
package m

import (
	"encoding/json"
	"fmt"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
)

func init() {
	event.Register(FullyReadEventType, parseFullyReadEvent)
}

// FullyReadEventType is the event type for m.fully_read.
const FullyReadEventType event.Type = "m.fully_read"

// FullyReadEvent describes the m.fully_read event.
type FullyReadEvent struct {
	// EventID is the event the user's read marker is located at in the room.
	EventID matrix.EventID `json:"event_id"`
	// RoomID is the room that the event read marker belongs to.
	RoomID matrix.RoomID `json:"-"`
}

func parseFullyReadEvent(raw event.RawEvent) (event.Event, error) {
	var ev FullyReadEvent
	if raw.Type != ev.Type() {
		return nil, fmt.Errorf("unexpected event type %q for FullyReadEvent", raw.Type)
	}

	if err := json.Unmarshal(raw.Content, &ev); err != nil {
		return nil, err
	}

	ev.RoomID = raw.RoomID
	return ev, nil
}

// Type implements event.Type.
func (ev FullyReadEvent) Type() event.Type { return FullyReadEventType }
