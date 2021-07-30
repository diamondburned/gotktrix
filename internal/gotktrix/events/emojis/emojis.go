// Package emojis provides an implementation of the im.ponies emoji protocol.
package emojis

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/pkg/errors"
)

// EmoticonEventData is a subevent struct that describes part of an emoji event.
type EmoticonEventData struct {
	Emoticons map[EmojiName]Emoji `json:"emoticons"`
}

// EmojiName describes the name of an emoji, which is surrounded by colons, such
// as ":gnutroll:".
type EmojiName string

// Name returns the emoji name without the colons.
func (n EmojiName) Name() string { return strings.Trim(string(n), ":") }

// Emoji describes the information of an emoji.
type Emoji struct {
	URL matrix.URL `json:"url"`
}

func init() {
	event.Register(RoomEmotesEventType, parseRoomEmotesEvent)
	event.Register(UserEmotesEventType, parseUserEmotesEvent)
}

const (
	RoomEmotesEventType event.Type = "im.ponies.room_emotes"
	UserEmotesEventType event.Type = "im.ponies.user_emotes"
)

// RoomEmotesEvent describes the im.ponies.room_emotes event.
type RoomEmotesEvent struct {
	event.RoomEventInfo
	EmoticonEventData
}

var _ event.StateEvent = (*RoomEmotesEvent)(nil)

func parseRoomEmotesEvent(raw event.RawEvent) (event.Event, error) {
	var ev RoomEmotesEvent
	if raw.Type != ev.Type() {
		return nil, fmt.Errorf("unexpected event type %q", raw.Type)
	}

	if err := json.Unmarshal(raw.Content, &ev); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal RoomEmotesEvent")
	}

	return ev, nil
}

func (ev RoomEmotesEvent) StateKey() string { return "" }

// Type implements event.Type.
func (ev RoomEmotesEvent) Type() event.Type { return RoomEmotesEventType }

// UserEmotesEvent describes the im.ponies.user_emotes event.
type UserEmotesEvent struct {
	EmoticonEventData
}

func parseUserEmotesEvent(raw event.RawEvent) (event.Event, error) {
	var ev UserEmotesEvent
	if raw.Type != ev.Type() {
		return nil, fmt.Errorf("unexpected event type %q for UserEmotesEvent", raw.Type)
	}

	if err := json.Unmarshal(raw.Content, &ev); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal UserEmotesEvent")
	}

	return ev, nil
}

// Type impleemnts event.Type.
func (ev UserEmotesEvent) Type() event.Type { return UserEmotesEventType }

// UserEmotes gets the current user's emojis.
func UserEmotes(c *gotktrix.Client) (UserEmotesEvent, error) {
	e, err := c.UserEvent(UserEmotesEventType)
	if err != nil {
		return UserEmotesEvent{}, err
	}

	return e.(UserEmotesEvent), nil
}

// RoomHasEmotes returns true if the room is known to have emojis.
func RoomHasEmotes(c *gotktrix.Client, roomID matrix.RoomID) bool {
	e, _ := RoomEmotes(c, roomID)
	return len(e.Emoticons) > 0
}

// RoomEmotes gets the room's emojis.
func RoomEmotes(c *gotktrix.Client, roomID matrix.RoomID) (RoomEmotesEvent, error) {
	e, err := c.RoomState(roomID, UserEmotesEventType, "")
	if err != nil {
		return RoomEmotesEvent{}, nil
	}

	return e.(RoomEmotesEvent), nil
}
