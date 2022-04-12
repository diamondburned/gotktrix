// Package sys works around the shortcomings of gotrix/event.
package sys

import (
	"encoding/json"
	"fmt"

	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

// Parse wraps around event.Parse.
func Parse(b []byte) event.Event {
	e, err := event.Parse(b)
	if err == nil {
		return e
	}

	return newErroneousEvent(b, err)
}

// ParseRoom wraps around event.Parse.
func ParseRoom(b []byte, rID matrix.RoomID) event.Event {
	e, err := event.Parse(append([]byte(nil), b...))
	if err == nil {
		room, ok := e.(event.RoomEvent)
		if ok {
			room.RoomInfo().RoomID = rID
		}
		return e
	}

	return newErroneousEvent(b, err)
}

// ParseTimeline wraps around event.Parse. TODO document.
func ParseTimeline(b []byte, rID matrix.RoomID) event.RoomEvent {
	e := ParseRoom(b, rID)

	if roomEv, ok := e.(event.RoomEvent); ok {
		return roomEv
	}

	// wat.
	return newErroneousEvent(b, fmt.Errorf("event %s in timeline is not RoomEvent", e.Info().Type))
}

// ParseAs returns a nil event if the returned event is erroneous or doesn't
// match the given type.
func ParseAs(b []byte, typ event.Type) (event.Event, error) {
	e, err := event.Parse(b)
	if err != nil {
		return nil, err
	}

	if typ != "" && e.Info().Type != typ {
		return nil, fmt.Errorf("event is not type %s, but %s", typ, e.Info().Type)
	}

	return e, nil
}

// ParseAll parses the given list of raw events into a new list of events.
func ParseAll(raws []event.RawEvent) []event.Event {
	evs := make([]event.Event, len(raws))
	for i, raw := range raws {
		evs[i] = Parse(raw)
	}
	return evs
}

// ParseAllRoom parses the given list of raw events into a new list of events.
func ParseAllRoom(raws []event.RawEvent, rID matrix.RoomID) []event.Event {
	evs := make([]event.Event, len(raws))
	for i, raw := range raws {
		evs[i] = ParseRoom(raw, rID)
	}
	return evs
}

// ParseAllTimeline parses the given list of raw events into a new list of room
// events.
func ParseAllTimeline(raws []event.RawEvent, rID matrix.RoomID) []event.RoomEvent {
	evs := make([]event.RoomEvent, len(raws))
	for i, raw := range raws {
		evs[i] = ParseTimeline(raw, rID)
	}
	return evs
}

type rawEvent struct {
	Type    event.Type      `json:"type"`
	Content json.RawMessage `json:"content"`
}

// ParseUserEventContent is used for user events only.
func ParseUserEventContent(typ event.Type, content json.RawMessage) (event.Event, error) {
	b, err := json.Marshal(rawEvent{
		Type:    typ,
		Content: content,
	})
	if err != nil {
		return nil, err
	}

	return ParseAs(b, typ)
}

// MarshalUserEvent is used for user events only. content must be valid JSON.
func MarshalUserEvent(typ event.Type, content json.RawMessage) []byte {
	b, err := json.Marshal(rawEvent{
		Type:    typ,
		Content: content,
	})
	if err != nil {
		panic("MarshalUserEvent: " + err.Error())
	}
	return b
}
