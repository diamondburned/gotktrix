package message

import (
	"context"
	"time"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

var _ = cssutil.WriteCSS(`
	.message-timestamp {
		font-size: 0.80em;
		color: alpha(@theme_fg_color, 0.55);
	}
	.message-collapsed {
		margin-right: 10px;
	}
	.message-collapsed .message-timestamp {
		min-height: 1.65em;
		opacity: 0;
	}
	.message-collapsed:hover .message-timestamp {
		opacity: 1;
	}
`)

// newTimestamp creates a new timestamp label. If long is true, then the label
// timestamp is long.
func newTimestamp(ts matrix.Timestamp, long bool) *gtk.Label {
	var t string
	if long {
		t = ts.Time().Format(time.Stamp)
	} else {
		t = ts.Time().Format(time.Kitchen)
	}

	l := gtk.NewLabel(t)
	l.SetTooltipText(ts.Time().Format(time.StampMilli))
	l.AddCSSClass("message-timestamp")

	return l
}

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
	timestamp := newTimestamp(v.raw.OriginServerTime, false)
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

type cozyMessage struct {
	*gtk.Box
	*eventBox
	parent messageViewer

	avatar    *adw.Avatar
	sender    *gtk.Label
	timestamp *gtk.Label
	content   *mcontent.Content
}

var _ = cssutil.WriteCSS(`
	.message-cozy {
		margin: 0px 10px;
		margin-top:  2px;
	}
	.message-cozy > box {
		margin-left: 10px;
	}
	.message-cozy .message-timestamp {
		margin-left: .5em;
	}
`)

func (v messageViewer) cozyMessage() *cozyMessage {
	client := v.client().Offline()

	nameLabel := gtk.NewLabel("")
	nameLabel.SetSingleLineMode(true)
	nameLabel.SetEllipsize(pango.EllipsizeEnd)
	nameLabel.SetMarkup(mauthor.Markup(
		client, v.raw.RoomID, v.raw.Sender,
		mauthor.WithWidgetColor(nameLabel),
	))

	timestamp := newTimestamp(v.raw.OriginServerTime, true)
	timestamp.SetEllipsize(pango.EllipsizeEnd)
	timestamp.SetYAlign(0.6)

	// Pull the username directly from the sender's ID for the avatar initials.
	username, _, _ := v.raw.Sender.Parse()

	avatar := adw.NewAvatar(avatarSize, username, true)
	avatar.SetVAlign(gtk.AlignStart)
	avatar.SetMarginTop(2)
	avatar.SetTooltipText(string(v.raw.Sender))

	mxc, _ := client.MemberAvatar(v.raw.RoomID, v.raw.Sender)
	if mxc != nil {
		setAvatar(v, avatar, client, *mxc)
	}

	authorTsBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	authorTsBox.Append(nameLabel)
	authorTsBox.Append(timestamp)

	content := mcontent.New(v.Context, v.raw)

	rightBox := gtk.NewBox(gtk.OrientationVertical, 0)
	rightBox.Append(authorTsBox)
	rightBox.Append(content)

	bigBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	bigBox.Append(&avatar.Widget)
	bigBox.Append(rightBox)

	bigBox.AddCSSClass("message-cozy")
	messageCSS(bigBox)

	msg := &cozyMessage{
		Box:      bigBox,
		eventBox: &eventBox{v.raw},
		parent:   v,

		avatar:    avatar,
		sender:    nameLabel,
		timestamp: timestamp,
		content:   content,
	}

	bindParent(v, msg, content)

	msg.asyncFetch()
	return msg
}

func (m *cozyMessage) asyncFetch() {
	opt := mauthor.WithWidgetColor(m.sender)

	go func() {
		markup := mauthor.Markup(m.parent.client(), m.parent.raw.RoomID, m.parent.raw.Sender, opt)
		glib.IdleAdd(func() { m.sender.SetMarkup(markup) })

		mxc, _ := m.parent.client().MemberAvatar(m.parent.raw.RoomID, m.parent.raw.Sender)
		if mxc != nil {
			setAvatar(m.parent, m.avatar, m.parent.client(), *mxc)
		}
	}()
}

// setAvatar is safe to be called concurrently.
func setAvatar(ctx context.Context, a *adw.Avatar, client *gotktrix.Client, mxc matrix.URL) {
	avatarURL, _ := client.SquareThumbnail(mxc, avatarSize)
	imgutil.AsyncGET(ctx, avatarURL, a.SetCustomImage)
}

// apiUpdate performs an asynchronous API update.
func (m *cozyMessage) apiUpdate() {
	// TODO
}
