package message

import (
	"context"
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
	// SetBlur greys the message's content if true. It's used to indicate
	// idling.
	SetBlur(bool)
	// RawEvent returns the raw unparsed room event.
	RawEvent() *gotktrix.EventBox
	// OnRelatedEvent is called by the caller for each event that's related to
	// the message. The caller should check the m.relates_to field.
	OnRelatedEvent(raw *gotktrix.EventBox)
}

func blurWidget(parent, content gtk.Widgetter, blur bool) {
	gtk.BaseWidget(content).SetSensitive(!blur)
	if blur {
		gtk.BaseWidget(parent).AddCSSClass("message-blurred")
	} else {
		gtk.BaseWidget(parent).RemoveCSSClass("message-blurred")
	}
}

type eventBox struct {
	*gotktrix.EventBox
}

func (b *eventBox) RawEvent() *gotktrix.EventBox {
	return b.EventBox
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

	raw *gotktrix.EventBox
}

func (v messageViewer) client() *gotktrix.Client {
	return gotktrix.FromContext(v)
}

// NewCozyMessage creates a new cozy or collapsed message.
func NewCozyMessage(ctx context.Context, view MessageViewer, raw *event.RawEvent) Message {
	viewer := messageViewer{
		Context:       ctx,
		MessageViewer: view,
		raw:           gotktrix.WrapEventBox(raw),
	}

	e, err := viewer.raw.Parse()
	if err != nil {
		return viewer.eventMessage()
	}

	if _, ok := e.(event.RoomMessageEvent); ok {
		if lastIsAuthor(view, raw) {
			return viewer.collapsedMessage()
		} else {
			return viewer.cozyMessage()
		}
	}

	return viewer.eventMessage()
}

// EditCozyMessage modifies v directly based on the underlying type that it has.
// The type of the message is preserved.
func EditCozyMessage(ctx context.Context, view MessageViewer, raw *event.RawEvent, v Message) {
	viewer := messageViewer{
		Context:       ctx,
		MessageViewer: view,
		raw:           gotktrix.WrapEventBox(raw),
	}

	_, err := viewer.raw.Parse()
	if err != nil {
		return
	}

	switch v := v.(type) {
	case *cozyMessage:
		*v = *viewer.cozyMessage()
	case *collapsedMessage:
		*v = *viewer.collapsedMessage()
	case *eventMessage:
		*v = *viewer.eventMessage()
	}
}

const maxCozyAge = 10 * time.Minute

func lastIsAuthor(view MessageViewer, ev *event.RawEvent) bool {
	// Ensure that the last message IS a cozy OR compact message.
	switch last := view.LastMessage().(type) {
	case *cozyMessage:
		return lastEventIsAuthor(last.EventBox.RawEvent, ev)
	case *collapsedMessage:
		return lastEventIsAuthor(last.EventBox.RawEvent, ev)
	default:
		return false
	}
}

func lastEventIsAuthor(last, ev *event.RawEvent) bool {
	return last.Sender == ev.Sender &&
		ev.OriginServerTime.Time().Sub(last.OriginServerTime.Time()) < maxCozyAge
}
