// Package emojis provides an implementation of the im.ponies emoji protocol.
package emojis

import (
	"encoding/json"
	"strings"

	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

// EmojiMap is a map (object) of emoji names to emoji objects.
type EmojiMap = map[EmojiName]Emoji

// EmoticonEventData is a subevent struct that describes part of an emoji event.
type EmoticonEventData struct {
	Emoticons EmojiMap `json:"emoticons"`
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
	event.RegisterDefault(RoomEmotesEventType, parseRoomEmotesEvent)
	event.RegisterDefault(UserEmotesEventType, parseUserEmotesEvent)
}

const (
	RoomEmotesEventType event.Type = "im.ponies.room_emotes"
	UserEmotesEventType event.Type = "im.ponies.user_emotes"
)

// RoomEmotesEvent describes the im.ponies.room_emotes event.
type RoomEmotesEvent struct {
	event.StateEventInfo `json:"-"`
	EmoticonEventData
}

var _ event.StateEvent = (*RoomEmotesEvent)(nil)

func parseRoomEmotesEvent(content json.RawMessage) (event.Event, error) {
	var ev RoomEmotesEvent
	err := json.Unmarshal(content, &ev)
	return &ev, err
}

// UserEmotesEvent describes the im.ponies.user_emotes event.
type UserEmotesEvent struct {
	event.EventInfo `json:"-"`
	EmoticonEventData
}

func parseUserEmotesEvent(content json.RawMessage) (event.Event, error) {
	var ev UserEmotesEvent
	err := json.Unmarshal(content, &ev)
	return &ev, err
}

// UserEmotes gets the current user's emojis.
func UserEmotes(c *gotktrix.Client) (EmojiMap, error) {
	e, err := c.UserEvent(UserEmotesEventType)
	if err != nil {
		return nil, err
	}

	ev, ok := e.(*UserEmotesEvent)
	if !ok {
		return nil, nil
	}

	return ev.Emoticons, nil
}

// RoomHasEmotes returns true if the room is known to have emojis.
func RoomHasEmotes(c *gotktrix.Client, roomID matrix.RoomID) bool {
	e, _ := RoomEmotes(c, roomID)
	return len(e) > 0
}

// RoomEmotes gets the room's emojis.
func RoomEmotes(c *gotktrix.Client, roomID matrix.RoomID) (EmojiMap, error) {
	e, err := c.RoomState(roomID, RoomEmotesEventType, "")
	if err != nil {
		return nil, err
	}

	ev, ok := e.(*RoomEmotesEvent)
	if !ok {
		return nil, nil
	}

	return ev.Emoticons, nil
}
