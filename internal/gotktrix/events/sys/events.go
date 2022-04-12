package sys

import (
	"encoding/json"

	"github.com/diamondburned/gotrix/event"
)

// ErroneousEvent is a pseudoevent returned by Parse to indicate events
// that cannot be parsed, probably because it's not known.
type ErroneousEvent struct {
	event.RoomEventInfo `json:"-"` // may be empty

	Err error
}

func newErroneousEvent(b []byte, err error) *ErroneousEvent {
	return &ErroneousEvent{newRoomEventInfo(b), err}
}

type partialType struct {
	Type event.Type `json:"type"`
}

func newRoomEventInfo(b []byte) event.RoomEventInfo {
	info := event.RoomEventInfo{
		EventInfo: event.EventInfo{Raw: b},
	}

	if err := json.Unmarshal(b, &info); err != nil {
		// Try and salvage the type field.
		var typ partialType
		json.Unmarshal(b, &typ)
		info.Type = typ.Type
	}

	return info
}

// IsRoomEvent returns true if the erroneous event is a room event.
func (ev ErroneousEvent) IsRoomEvent() bool { return ev.ID != "" }

// String returns the event name or a placeholder.
func (ev ErroneousEvent) String() string {
	if ev.Type != "" {
		return string(ev.Type)
	}
	return "<no event type>"
}

// Error implements error.
func (ev ErroneousEvent) Error() string {
	return ev.Err.Error()
}
