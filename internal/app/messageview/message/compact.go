package message

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

// collapsedMessage is part of the full message container.
type collapsedMessage struct {
	*gtk.Box
	*message
}

const (
	avatarSize  = 36
	avatarWidth = avatarSize + 10*2 // keep consistent with CSS
)

var compactCSS = cssutil.Applier("message-collapsed", `
	.message-collapsed .message-timestamp {
		font-size:  .65em;
		min-height: 1.9em;
	}
	.message-collapsed:not(.message-collapsed-edited) .message-timestamp {
		opacity: 0;
	}
	.message-collapsed:hover .message-timestamp {
		opacity: 1;
	}
`)

func (v messageViewer) collapsedMessage() *collapsedMessage {
	msg := v.newMessage(false)
	msg.timestamp.SetSizeRequest(avatarWidth, -1)
	msg.timestamp.SetVAlign(gtk.AlignStart)
	msg.timestamp.SetYAlign(1)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(msg.timestamp)
	box.Append(msg.content)

	box.AddCSSClass("message-collapsed")
	messageCSS(box)

	bindParent(v, box, msg.content)

	return &collapsedMessage{
		Box:     box,
		message: msg,
	}
}

func (m *collapsedMessage) SetBlur(blur bool) {
	m.message.setBlur(m, blur)
}

func (m *collapsedMessage) OnRelatedEvent(ev *gotktrix.EventBox) {
	m.message.OnRelatedEvent(ev)

	_, edited := m.content.EditedTimestamp()
	if edited {
		m.AddCSSClass("message-collapsed-edited")
		m.timestamp.SetText(locale.S(m.parent, "(edited)"))
	}
}
