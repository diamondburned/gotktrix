package message

import (
	"context"
	"time"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// Message describes a generic message type.
type Message interface {
	gtk.Widgetter
	// SetBlur greys the message's content if true. It's used to indicate
	// idling.
	SetBlur(bool)
	// Event returns the message's event.
	Event() event.RoomEvent
	// OnRelatedEvent is called by the caller for each event that's related to
	// the message. The caller should check the m.relates_to field.
	OnRelatedEvent(event.RoomEvent)
	// LoadMore loads more information in the message, such as embeds. It should
	// be synchronous most of the time.
	LoadMore()
}

var messageCSS = cssutil.Applier("message-message", `
	/* .message-collapsed */
	/* .message-cozy */
	/* .message-event */

	.message-message {
		margin: 0;
		padding-right: 8px;
		border-left: 2px solid transparent;
	}
	.message-mentions {
		border-left: 2px solid @theme_selected_bg_color;
		background-color: alpha(@theme_selected_bg_color, 0.05);
	}
`)

// MessageViewer describes the parent that holds messages.
type MessageViewer interface {
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
	event event.RoomEvent
}

func (v messageViewer) client() *gotktrix.Client {
	return gotktrix.FromContext(v)
}

// TODO: API improvements:
//  - have a single NewMessage that uses a global setting in the future
//  - give NewMessage a message mark
//  - have MessageViewer.BeforeMessage take a mark to grab the previous message,
//    if needed

// NewCozyMessage creates a new cozy or collapsed message.
func NewCozyMessage(ctx context.Context, view MessageViewer, ev event.RoomEvent, before Message) Message {
	viewer := messageViewer{
		Context:       ctx,
		MessageViewer: view,
		event:         ev,
	}

	var message Message

	if ev, ok := ev.(*event.RoomMessageEvent); ok {
		if lastIsAuthor(before, ev) {
			message = viewer.collapsedMessage(ev)
		} else {
			message = viewer.cozyMessage(ev)
		}

		client := viewer.client()

		if client.NotifyMessage(ev, gotktrix.HighlightMessage) != 0 {
			w := gtk.BaseWidget(message)
			w.AddCSSClass("message-mentions")
		}
	} else {
		message = viewer.eventMessage()
	}

	return message
}

const maxCozyAge = 10 * time.Minute

func lastIsAuthor(before Message, ev *event.RoomMessageEvent) bool {
	// Ensure that the last message IS a cozy OR compact message.
	switch before := before.(type) {
	case *cozyMessage, *collapsedMessage:
		last := before.Event().RoomInfo()
		return last.Sender == ev.Sender &&
			ev.OriginServerTime.Time().Sub(last.OriginServerTime.Time()) < maxCozyAge
	default:
		return false
	}
}

// message is the base message type that other message types can compose upon.
type message struct {
	parent    messageViewer
	timestamp *timestamp
	content   *mcontent.Content
}

func (v messageViewer) newMessage(ev *event.RoomMessageEvent, longTimestamp bool) *message {
	timestamp := newTimestamp(v, v.event.RoomInfo().OriginServerTime.Time(), longTimestamp)
	timestamp.SetEllipsize(pango.EllipsizeEnd)

	return &message{
		parent:    v,
		timestamp: timestamp,
		content:   mcontent.New(v.Context, ev),
	}
}

func (m *message) Event() event.RoomEvent {
	return m.parent.event
}

func (m *message) OnRelatedEvent(ev event.RoomEvent) {
	m.content.OnRelatedEvent(ev)

	t, edited := m.content.EditedTimestamp()
	if edited {
		m.timestamp.setEdited(t.Time())
	}
}

func (m *message) LoadMore() {
	m.content.LoadMore()
}

func (m *message) setBlur(parent gtk.Widgetter, blur bool) {
	gtk.BaseWidget(m.content).SetSensitive(!blur)
	setBlurClass(m.content, blur)
}

func setBlurClass(w gtk.Widgetter, blur bool) {
	if blur {
		gtk.BaseWidget(w).AddCSSClass("message-blurred")
	} else {
		gtk.BaseWidget(w).RemoveCSSClass("message-blurred")
	}
}
