package message

import (
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
)

type cozyMessage struct {
	*gtk.Box
	*message
	avatar *onlineimage.Avatar
	sender *gtk.Label
}

const (
	avatarSize  = 36
	avatarWidth = avatarSize + 8*2 // keep consistent with CSS
)

var _ = cssutil.WriteCSS(`
	.message-cozy {
		padding-top:    4px;
		padding-bottom: 2px;
	}
	.message-cozy > box {
		margin-left: 8px;
	}
	.message-cozy .message-timestamp {
		margin-left: .5em;
	}
`)

func (v messageViewer) cozyMessage(ev *event.RoomMessageEvent) *cozyMessage {
	client := v.client().Offline()

	msg := cozyMessage{}
	msg.message = v.newMessage(ev, true)
	msg.message.timestamp.SetYAlign(1)

	msg.sender = gtk.NewLabel("")
	msg.sender.SetTooltipText(string(ev.Sender))
	msg.sender.SetSingleLineMode(true)
	msg.sender.SetEllipsize(pango.EllipsizeEnd)
	msg.sender.SetMarkup(mauthor.Markup(
		client, ev.RoomID, ev.Sender,
		mauthor.WithWidgetColor(),
	))

	msg.avatar = onlineimage.NewAvatar(v, gotktrix.AvatarProvider, avatarSize)
	msg.avatar.ConnectLabel(msg.sender)
	msg.avatar.SetVAlign(gtk.AlignStart)
	msg.avatar.SetMarginTop(2)
	msg.avatar.SetTooltipText(string(ev.Sender))

	mxc, _ := client.MemberAvatar(ev.RoomID, ev.Sender)
	if mxc != nil {
		msg.avatar.SetFromURL(string(*mxc))
	}

	authorTsBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	authorTsBox.Append(msg.sender)
	authorTsBox.Append(msg.timestamp)

	rightBox := gtk.NewBox(gtk.OrientationVertical, 0)
	rightBox.Append(authorTsBox)
	rightBox.Append(msg.content)

	msg.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	msg.Box.Append(msg.avatar)
	msg.Box.Append(rightBox)

	msg.AddCSSClass("message-cozy")
	messageCSS(msg)

	bindParent(v, msg, msg.content)
	return &msg
}

func (m *cozyMessage) SetBlur(blur bool) {
	m.message.setBlur(m, blur)
}

func (m *cozyMessage) LoadMore() {
	m.asyncFetch()
	m.message.LoadMore()
}

func (m *cozyMessage) asyncFetch() {
	opt := mauthor.WithWidgetColor()

	roomEv := m.parent.event.RoomInfo()
	go func() {
		markup := mauthor.Markup(m.parent.client(), roomEv.RoomID, roomEv.Sender, opt)
		glib.IdleAdd(func() { m.sender.SetMarkup(markup) })

		mxc, _ := m.parent.client().MemberAvatar(roomEv.RoomID, roomEv.Sender)
		if mxc != nil {
			glib.IdleAdd(func() { m.avatar.SetFromURL(string(*mxc)) })
		}
	}()
}
