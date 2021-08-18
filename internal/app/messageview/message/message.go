package message

import (
	"context"
	"fmt"
	"time"

	"github.com/chanbakjsd/gotrix/event"
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

	bind(ctx, msg)
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
