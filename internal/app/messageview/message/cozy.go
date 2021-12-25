package message

import (
	"context"
	"log"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

type cozyMessage struct {
	*gtk.Box
	*message
	avatar *adaptive.Avatar
	sender *gtk.Label
}

var _ = cssutil.WriteCSS(`
	.message-cozy {
		padding-top:    4px;
		padding-bottom: 2px;
	}
	.message-cozy > box {
		margin-left: 10px;
	}
	.message-cozy .message-timestamp {
		margin-left: .5em;
	}
`)

func (v messageViewer) cozyMessage(ev *event.RoomMessageEvent) *cozyMessage {
	client := v.client().Offline()

	msg := cozyMessage{}
	msg.message = v.newMessage(ev, true)
	msg.message.timestamp.SetYAlign(0.6)

	msg.sender = gtk.NewLabel("")
	msg.sender.SetTooltipText(string(ev.Sender))
	msg.sender.SetSingleLineMode(true)
	msg.sender.SetEllipsize(pango.EllipsizeEnd)
	msg.sender.SetMarkup(mauthor.Markup(
		client, ev.RoomID, ev.Sender,
		mauthor.WithWidgetColor(msg.sender),
	))

	msg.avatar = adaptive.NewAvatar(avatarSize)
	msg.avatar.ConnectLabel(msg.sender)
	msg.avatar.SetVAlign(gtk.AlignStart)
	msg.avatar.SetMarginTop(2)
	msg.avatar.SetTooltipText(string(ev.Sender))

	mxc, _ := client.MemberAvatar(ev.RoomID, ev.Sender)
	if mxc != nil {
		setAvatar(v, msg.avatar, client, *mxc)
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
	opt := mauthor.WithWidgetColor(m.sender)

	roomEv := m.parent.event.RoomInfo()
	go func() {
		markup := mauthor.Markup(m.parent.client(), roomEv.RoomID, roomEv.Sender, opt)
		glib.IdleAdd(func() { m.sender.SetMarkup(markup) })

		mxc, _ := m.parent.client().MemberAvatar(roomEv.RoomID, roomEv.Sender)
		if mxc != nil {
			setAvatar(m.parent, m.avatar, m.parent.client(), *mxc)
		}
	}()
}

// setAvatar is safe to be called concurrently.
func setAvatar(ctx context.Context, a *adaptive.Avatar, client *gotktrix.Client, mxc matrix.URL) {
	avatarURL, _ := client.SquareThumbnail(mxc, avatarSize, gtkutil.ScaleFactor())
	imgutil.AsyncGET(
		ctx, avatarURL, a.SetFromPaintable,
		imgutil.WithErrorFn(func(err error) {
			log.Print("error getting avatar ", mxc, ": ", err)
		}),
	)
}
