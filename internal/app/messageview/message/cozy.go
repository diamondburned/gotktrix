package message

import (
	"context"
	"log"
	"time"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

var _ = cssutil.WriteCSS(`
	.message-timestamp {
		font-size: 0.80em;
		color: alpha(@theme_fg_color, 0.55);
	}
	.message-message {
		margin-right: 8px;
	}
	.message-collapsed .message-timestamp {
		opacity: 0;
		font-size: .65em;
		min-height: 1.9em;
	}
	.message-collapsed:hover .message-timestamp {
		opacity: 1;
	}
`)

// newTimestamp creates a new timestamp label. If long is true, then the label
// timestamp is long.
func newTimestamp(ctx context.Context, ts time.Time, long bool) *gtk.Label {
	var t string
	if long {
		t = locale.TimeAgo(ctx, ts)
	} else {
		t = locale.Time(ts, false)
	}

	l := gtk.NewLabel(t)
	l.SetTooltipText(ts.Format(time.StampMilli))
	l.AddCSSClass("message-timestamp")

	return l
}

type cozyMessage struct {
	*gtk.Box
	*eventBox
	parent messageViewer

	avatar    *adaptive.Avatar
	sender    *gtk.Label
	timestamp *gtk.Label
	content   *mcontent.Content
}

var _ = cssutil.WriteCSS(`
	.message-cozy {
		margin-top: 2px;
		margin-bottom: 0;
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
	nameLabel.SetTooltipText(string(v.raw.Sender))
	nameLabel.SetSingleLineMode(true)
	nameLabel.SetEllipsize(pango.EllipsizeEnd)
	nameLabel.SetMarkup(mauthor.Markup(
		client, v.raw.RoomID, v.raw.Sender,
		mauthor.WithWidgetColor(nameLabel),
	))

	timestamp := newTimestamp(v, v.raw.OriginServerTime.Time(), true)
	timestamp.SetEllipsize(pango.EllipsizeEnd)
	timestamp.SetYAlign(0.6)

	avatar := adaptive.NewAvatar(avatarSize)
	avatar.ConnectLabel(nameLabel)
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
	return msg
}

func (m *cozyMessage) SetBlur(blur bool) {
	blurWidget(m, m.content, blur)
}

func (m *cozyMessage) OnRelatedEvent(ev *gotktrix.EventBox) {
	m.content.OnRelatedEvent(ev)
}

func (m *cozyMessage) LoadMore() {
	m.asyncFetch()
	m.content.LoadMore()
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
func setAvatar(ctx context.Context, a *adaptive.Avatar, client *gotktrix.Client, mxc matrix.URL) {
	avatarURL, _ := client.SquareThumbnail(mxc, avatarSize, gtkutil.ScaleFactor())
	imgutil.AsyncGET(
		ctx, avatarURL, a.SetFromPaintable,
		imgutil.WithErrorFn(func(err error) {
			log.Print("error getting avatar ", mxc, ": ", err)
		}),
	)
}

// apiUpdate performs an asynchronous API update.
func (m *cozyMessage) apiUpdate() {
	// TODO
}
