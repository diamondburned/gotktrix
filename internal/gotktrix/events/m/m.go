// Package m provides Matrix-namespace events.
package m

import (
	"encoding/json"
	"fmt"

	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

func init() {
	event.Register(FullyReadEventType, parseFullyReadEvent)
	event.RegisterDefault(SpaceChildEventType, parseSpaceChildEvent)
	event.RegisterDefault(SpaceParentEventType, parseSpaceParentEvent)
	event.RegisterDefault(ReactionEventType, parseReactionEvent)
}

// FullyReadEventType is the event type for m.fully_read.
const FullyReadEventType event.Type = "m.fully_read"

// FullyReadEventInfo is the information outside the content piece of
// FullyReadEvent.
type FullyReadEventInfo struct {
	event.EventInfo
	// RoomID is the room that the event read marker belongs to.
	RoomID matrix.RoomID `json:"room_id"`
}

// FullyReadEvent describes the m.fully_read event.
type FullyReadEvent struct {
	FullyReadEventInfo `json:"-"`
	// EventID is the event the user's read marker is located at in the room.
	EventID matrix.EventID `json:"event_id"`
}

func parseFullyReadEvent(whole event.RawEvent, content json.RawMessage) (event.Event, error) {
	var ev FullyReadEvent
	if err := json.Unmarshal(content, &ev); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(whole, &ev.FullyReadEventInfo); err != nil {
		return nil, err
	}
	return &ev, nil
}

// MarshalFullyReadEvent marshals the given fully read event.
func MarshalFullyReadEvent(ev FullyReadEvent) event.RawEvent {
	// Ensure type field is set.
	ev.FullyReadEventInfo.Type = FullyReadEventType

	raw := struct {
		*FullyReadEventInfo
		Content *FullyReadEvent `json:"content"`
	}{
		FullyReadEventInfo: &ev.FullyReadEventInfo,
		Content:            &ev,
	}

	b, err := json.Marshal(raw)
	if err != nil {
		panic("cannot marshal m.fully_read: " + err.Error())
	}

	return b
}

// RelType is the type for the "m.relates_to".rel_type field.
type RelType string

const (
	Annotation RelType = "m.annotation"
	Replace    RelType = "m.replace"
)

// ReactionEventType is the event type for m.reaction.
const ReactionEventType event.Type = "m.reaction"

// ReactionEvent is a reaction event of type m.reaction.
type ReactionEvent struct {
	event.RoomEventInfo `json:"-"`

	RelatesTo ReactionRelatesTo `json:"m.relates_to"`
}

// ReactionRelatesTo is the type of the relates_to object inside an m.reaction.
type ReactionRelatesTo struct {
	RelType RelType        `json:"rel_type"` // often m.annotation
	EventID matrix.EventID `json:"event_id"`
	Key     string         `json:"key"`
}

func parseReactionEvent(content json.RawMessage) (event.Event, error) {
	var ev ReactionEvent
	err := json.Unmarshal(content, &ev)
	return &ev, err
}

// SpaceChildEventType is the event type for m.space.child.
const SpaceChildEventType = "m.space.child"

// SpaceChildEvent is an event emitted by space rooms to advertise children
// rooms.
type SpaceChildEvent struct {
	event.StateEventInfo `json:"-"`

	Via       []string `json:"via"`
	Order     string   `json:"order,omitempty"`
	Suggested bool     `json:"suggested,omitempty"`
}

func parseSpaceChildEvent(content json.RawMessage) (event.Event, error) {
	var ev SpaceChildEvent
	err := json.Unmarshal(content, &ev)
	return &ev, err
}

// ChildRoomID returns the room ID that this space child event describes.
func (ev *SpaceChildEvent) ChildRoomID() matrix.RoomID {
	return matrix.RoomID(ev.StateEventInfo.StateKey)
}

// SpaceParentEventType is the event type for m.space.parent.
const SpaceParentEventType = "m.space.parent"

// SpaceParentEvent is an event emitted by children rooms to advertise spaces.
type SpaceParentEvent struct {
	event.StateEventInfo `json:"-"`

	Via       []string `json:"via"`
	Canonical bool     `json:"canonical,omitempty"`
}

func parseSpaceParentEvent(content json.RawMessage) (event.Event, error) {
	var ev SpaceParentEvent
	err := json.Unmarshal(content, &ev)
	return &ev, err
}

// SpaceRoomID returns the room ID that this space child event describes.
func (ev *SpaceParentEvent) SpaceRoomID() matrix.RoomID {
	return matrix.RoomID(ev.StateEventInfo.StateKey)
}

// DiscordMember describes a Discord member, which sits inside a field labeled
// "uk.half-shot.discord.member" in the RoomMemberEvent.
type DiscordMember struct {
	ID           uint64        `json:"id,string"`
	Username     string        `json:"username"`
	Roles        []DiscordRole `json:"roles"`
	DisplayColor uint32        `json:"displayColor"`
	Bot          bool          `json:"bot"`
}

// DisplayHexColor returns the DisplayColor in hexadecimal w/ the # prefix.
func (m DiscordMember) DisplayHexColor() string {
	return fmt.Sprintf("#%06X", m.DisplayColor)
}

// DiscordRole describes the role of a Discord user.
type DiscordRole struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
	Color    uint32 `json:"color"`
}

// DiscordMemberFromMatrix returns the DiscordMember of the given Matrix member
// event, if any.
func DiscordMemberFromMatrix(m *event.RoomMemberEvent) *DiscordMember {
	raw := m.Info().Raw
	if raw == nil {
		// No raw, so we can't read the intended field.
		return nil
	}

	var member struct {
		Content struct {
			DiscordMember *DiscordMember `json:"uk.half-shot.discord.member"`
		} `json:"content"`
	}

	json.Unmarshal(raw, &member)
	return member.Content.DiscordMember
}

// NotificationCount is the struct type inside
// api.SyncJoinedRoomEvents.UnreadCount.
type NotificationCount struct {
	Highlight    int `json:"highlight_count,omitempty"`
	Notification int `json:"notification_count,omitempty"`
}
