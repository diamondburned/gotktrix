package message

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
)

// collapsedMessage is part of the full message container.
type collapsedMessage struct {
	*gtk.Box
	*eventBox

	timestamp *gtk.Label
	content   *mcontent.Content
}

const (
	avatarSize  = 36
	avatarWidth = avatarSize + 10*2 // keep consistent with CSS
)

func (v messageViewer) collapsedMessage() *collapsedMessage {
	timestamp := newTimestamp(v, v.raw.OriginServerTime.Time(), false)
	timestamp.SetSizeRequest(avatarWidth, -1)
	timestamp.SetVAlign(gtk.AlignStart)
	timestamp.SetYAlign(1)

	content := mcontent.New(v.Context, v.raw)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(timestamp)
	box.Append(content)

	box.AddCSSClass("message-collapsed")
	messageCSS(box)

	bindParent(v, box, content)

	return &collapsedMessage{
		Box:      box,
		eventBox: &eventBox{v.raw},

		timestamp: timestamp,
		content:   content,
	}
}

func (m *collapsedMessage) SetBlur(blur bool) {
	blurWidget(m, m.content, blur)
}

func (m *collapsedMessage) OnRelatedEvent(ev *gotktrix.EventBox) {
	m.content.OnRelatedEvent(ev)
}

func (m *collapsedMessage) LoadMore() {
	m.content.LoadMore()
}
