package message

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotrix/event"
)

// collapsedMessage is part of the full message container.
type collapsedMessage struct {
	*gtk.Box
	*message
}

var compactCSS = cssutil.Applier("message-collapsed", `
	.message-collapsed {
		padding-top:    2px;
		padding-bottom: 2px;
	}
	.message-collapsed .message-timestamp {
		font-size:  0.65em;
		min-height: 1.9em;
	}
	.message-collapsed:not(.message-collapsed-edited) .message-timestamp {
		opacity: 0;
	}
	.message-collapsed:hover .message-timestamp {
		opacity: 1;
	}
`)

func (v messageViewer) collapsedMessage(ev *event.RoomMessageEvent) *collapsedMessage {
	msg := v.newMessage(ev, false)
	msg.timestamp.SetSizeRequest(avatarWidth, -1)
	// Actually make ellipsizing work.
	msg.timestamp.SetLayoutManager(gtk.NewFixedLayout())
	msg.timestamp.SetVAlign(gtk.AlignStart)
	msg.timestamp.SetYAlign(1)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(msg.timestamp)
	box.Append(msg.content)

	messageCSS(box)
	compactCSS(box)

	bindParent(v, box, msg.content)

	return &collapsedMessage{
		Box:     box,
		message: msg,
	}
}

func (m *collapsedMessage) SetBlur(blur bool) {
	m.message.setBlur(m, blur)
}

func (m *collapsedMessage) OnRelatedEvent(ev event.RoomEvent) bool {
	ok := m.message.OnRelatedEvent(ev)

	_, edited := m.content.EditedTimestamp()
	if edited {
		m.AddCSSClass("message-collapsed-edited")
		m.timestamp.SetText(locale.S(m.parent, "(edited)"))
	}

	return ok
}
