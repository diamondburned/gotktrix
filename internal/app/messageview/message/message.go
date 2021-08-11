package message

import (
	"context"
	"time"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// Message describes a generic message type.
type Message interface {
	gtk.Widgetter
	// Event returns the origin room event.
	Event() event.RoomEvent
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

// TODO
// type compactMessage struct{
// 	content *gtk.TextView
// }

// messageViewer fuses MessageViewer into Context. It's only used internally;
// doing this publicly is quite ugly.
type messageViewer struct {
	MessageViewer
	context.Context
}

func (v messageViewer) client() *gotktrix.Client {
	return gotktrix.FromContext(v)
}

// NewCozyMessage creates a new cozy or collapsed message.
func NewCozyMessage(ctx context.Context, view MessageViewer, ev event.RoomEvent) Message {
	viewer := messageViewer{
		Context:       ctx,
		MessageViewer: view,
	}

	var msg Message

	switch ev := ev.(type) {
	case event.RoomMessageEvent:
		if lastIsAuthor(view, ev) {
			msg = viewer.CollapsedMessage(&ev)
		} else {
			msg = viewer.CozyMessage(&ev)
		}
	default:
		msg = viewer.EventMessage(ev)
	}

	bind(ctx, msg)
	return msg
}

const maxCozyAge = 10 * time.Minute

func lastIsAuthor(view MessageViewer, ev event.RoomMessageEvent) bool {
	last := view.LastMessage()
	// Ensure that the last message IS a cozy OR compact message.
	switch last := last.(type) {
	case *cozyMessage:
		return lastEventIsAuthor(last.ev, &ev)
	case *collapsedMessage:
		return lastEventIsAuthor(last.ev, &ev)
	default:
		return false
	}
}

func lastEventIsAuthor(last, ev *event.RoomMessageEvent) bool {
	return last.SenderID == ev.SenderID &&
		ev.OriginTime.Time().Sub(last.OriginTime.Time()) < maxCozyAge
}
