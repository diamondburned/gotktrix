package message

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// Message describes a generic message type.
type Message interface {
	gtk.Widgetter
	// Event returns the original room event.
	Event() event.RoomEvent
	// RawEvent returns the raw unparsed room event.
	RawEvent() *event.RawEvent
}

var messageCSS = cssutil.Applier("message-message", `
	/* .message-collapsed */
	/* .message-cozy */
	/* .message-event */
`)

// MessageViewer describes the parent that holds messages.
type MessageViewer interface {
	// LastMessage returns the latest message.
	LastMessage() Message
	// ReplyTo sets the message event ID that the user wants to reply to.
	ReplyTo(matrix.EventID)
	// Edit starts the editing for given message ID.
	Edit(matrix.EventID)
}

// messageViewer fuses MessageViewer into Context. It's only used internally;
// doing this publicly is quite ugly.
type messageViewer struct {
	MessageViewer
	context.Context

	raw *event.RawEvent
}

func (v messageViewer) client() *gotktrix.Client {
	return gotktrix.FromContext(v)
}

type eventBox struct {
	raw *event.RawEvent
	ev  event.RoomEvent
}

func (b eventBox) Event() event.RoomEvent    { return b.ev }
func (b eventBox) RawEvent() *event.RawEvent { return b.raw }

// NewCozyMessage creates a new cozy or collapsed message.
func NewCozyMessage(ctx context.Context, view MessageViewer, raw *event.RawEvent) Message {
	viewer := messageViewer{
		Context:       ctx,
		MessageViewer: view,

		raw: raw,
	}

	var msg Message

	e, err := raw.Parse()
	if err != nil {
		return viewer.eventMessage(eventBox{
			raw: raw,
			ev:  WrapErroneousEvent(raw, err),
		})
	}

	ev, ok := e.(event.RoomEvent)
	if !ok {
		return viewer.eventMessage(eventBox{
			raw: raw,
			ev:  WrapErroneousEvent(raw, fmt.Errorf("event %T is not a room event", e)),
		})
	}

	evbox := eventBox{
		raw: raw,
		ev:  ev,
	}

	if _, ok := e.(event.RoomMessageEvent); ok {
		if lastIsAuthor(view, raw) {
			msg = viewer.collapsedMessage(evbox)
		} else {
			msg = viewer.cozyMessage(evbox)
		}
	} else {
		msg = viewer.eventMessage(evbox)
	}

	bind(viewer, msg)
	return msg
}

const maxCozyAge = 10 * time.Minute

func lastIsAuthor(view MessageViewer, ev *event.RawEvent) bool {
	last := view.LastMessage()

	// Ensure that the last message IS a cozy OR compact message.
	switch last.(type) {
	case *cozyMessage, *collapsedMessage:
		// ok
	default:
		return false
	}

	return lastEventIsAuthor(last.RawEvent(), ev)
}

func lastEventIsAuthor(last, ev *event.RawEvent) bool {
	return last.Sender == ev.Sender &&
		ev.OriginServerTime.Time().Sub(last.OriginServerTime.Time()) < maxCozyAge
}

// EditedMessage describes an edited message.
type EditedMessage struct {
	event.RawEvent
	Replaces matrix.EventID `json:"-"`
}

var editKeep = map[string]struct{}{
	"m.relates_to":  {},
	"m.new_content": {},
}

// Edited checks if ev is a RoomMessageEvent that is an edit of another message.
// If that's the case, then a copy of ev is returned with the edited content in
// place. Otherwise, nil is returned.
func Edited(ev *event.RawEvent) *EditedMessage {
	var relatesToEv struct {
		RelatesTo struct {
			RelType string         `json:"rel_type"`
			EventID matrix.EventID `json:"event_id"`
		} `json:"m.relates_to"`
	}

	if err := json.Unmarshal(ev.Content, &relatesToEv); err != nil {
		return nil
	}

	if relatesToEv.RelatesTo.RelType != "m.replace" {
		return nil
	}

	content := map[string]interface{}{}
	if err := json.Unmarshal(ev.Content, &content); err != nil {
		return nil
	}

	edited := EditedMessage{RawEvent: *ev}
	edited.Replaces = relatesToEv.RelatesTo.EventID

	// Replace fields in a lossless way by using a generic
	// map[string]interface{} type.
	new, ok := content["m.new_content"].(map[string]interface{})
	if !ok {
		// We don't have a m.new_content field for some reason. Pretend that
		// this message isn't an edited message, since it's invalid.
		return nil
	}

	for k, v := range new {
		// Don't override certain fields, since we might use them.
		if _, ok := editKeep[k]; ok {
			continue
		}
		// Override other fields from NewContent into this.
		content[k] = v
	}

	b, err := json.Marshal(content)
	if err != nil {
		log.Panicln("failed to remarshal content after parsing:", err)
	}

	// Save the content.
	edited.RawEvent.Content = b

	return &edited
}
