package message

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/gotk3/gotk3/glib"
)

// Message describes a generic message type.
type Message interface {
	gtk.Widgetter
	// Event returns the origin room event.
	Event() event.RoomEvent
}

// MessageViewer describes the parent that holds messages.
type MessageViewer interface {
	// LastMessage returns the latest message.
	LastMessage() Message
	// Context returns the viewer's context that is associated with the client.
	Context() context.Context
	// Client returns the Matrix client associated with the viewer. The viewer
	// must only return a constant client at all times.
	Client() *gotktrix.Client
}

// TODO
// type compactMessage struct{
// 	content *gtk.TextView
// }

// NewCozyMessage creates a new cozy or collapsed message.
func NewCozyMessage(parent MessageViewer, ev event.RoomEvent) Message {
	switch ev := ev.(type) {
	case event.RoomMessageEvent:
		var msg Message

		if lastIsAuthor(parent, ev) {
			msg = newCollapsedMessage(parent, &ev)
		} else {
			msg = newCozyMessage(parent, &ev)
		}

		return msg
	default:
		return newEventMessage(parent, ev)
	}
}

func lastIsAuthor(parent MessageViewer, ev event.RoomMessageEvent) bool {
	last := parent.LastMessage()
	return last != nil && last.Event().Sender() == ev.SenderID
}

var messageCSS = cssutil.Applier("message-message", `
	/* .message-collapsed */
	/* .message-cozy */
	/* .message-event */
`)

// eventMessage is a mini-message.
type eventMessage struct {
	*gtk.Label
	ev event.RoomEvent
}

var _ = cssutil.WriteCSS(`
	.message-event {
		font-size: .9em;
		color: alpha(@theme_fg_color, 0.8);
	}
`)

func newEventMessage(parent MessageViewer, ev event.RoomEvent) Message {
	action := gtk.NewLabel("")
	action.AddCSSClass("message-event")
	action.SetWrap(true)
	action.SetWrapMode(pango.WrapWordChar)

	markup := mauthor.Markup(
		parent.Client().Offline(), ev.Room(), ev.Sender(),
		mauthor.WithWidgetColor(action),
	)

	action.SetMarkup(fmt.Sprintf(
		"%s did an event %T (%s).",
		markup, ev, html.EscapeString(string(ev.ID())),
	))

	messageCSS(action)

	return &eventMessage{
		Label: action,
		ev:    ev,
	}
}

func (m *eventMessage) Event() event.RoomEvent { return m.ev }

var _ = cssutil.WriteCSS(`
	.message-timestamp {
		font-size: .8em;
		color: alpha(@theme_fg_color, 0.55);
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
	l.AddCSSClass("message-timestamp")

	return l
}

// collapsedMessage is part of the full message container.
type collapsedMessage struct {
	*gtk.Box
	ev *event.RoomMessageEvent

	timestamp *gtk.Label
	content   *mcontent.Content
}

const (
	avatarSize  = 36
	avatarWidth = 36 + 10*2 // keep consistent with CSS
)

func newCollapsedMessage(parent MessageViewer, ev *event.RoomMessageEvent) Message {
	client := parent.Client().Offline()

	timestamp := newTimestamp(ev.OriginTime, false)
	timestamp.SetSizeRequest(avatarWidth, -1)

	content := mcontent.New(client, ev)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(timestamp)
	box.Append(content)

	box.AddCSSClass("message-collapsed")
	messageCSS(box)

	return &collapsedMessage{
		Box: box,
		ev:  ev,

		timestamp: timestamp,
		content:   content,
	}
}

func (m *collapsedMessage) Event() event.RoomEvent { return *m.ev }

type cozyMessage struct {
	*gtk.Box
	ev     *event.RoomMessageEvent
	parent MessageViewer

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

func newCozyMessage(parent MessageViewer, ev *event.RoomMessageEvent) Message {
	client := parent.Client().Offline()

	nameLabel := gtk.NewLabel("")
	nameLabel.SetSingleLineMode(true)
	nameLabel.SetEllipsize(pango.EllipsizeEnd)
	nameLabel.SetMarkup(mauthor.Markup(
		client, ev.RoomID, ev.SenderID,
		mauthor.WithWidgetColor(nameLabel),
	))

	timestamp := newTimestamp(ev.OriginTime, true)
	timestamp.SetEllipsize(pango.EllipsizeEnd)
	timestamp.SetYAlign(0.6)

	// Pull the username directly from the sender's ID for the avatar initials.
	username, _, _ := ev.SenderID.Parse()

	avatar := adw.NewAvatar(avatarSize, username, true)
	avatar.SetVAlign(gtk.AlignStart)
	avatar.SetMarginTop(2)

	mxc, _ := client.AvatarURL(ev.SenderID)
	if mxc != nil {
		setAvatar(parent.Context(), avatar, client, *mxc)
	}

	authorTsBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	authorTsBox.Append(nameLabel)
	authorTsBox.Append(timestamp)

	content := mcontent.New(client, ev)

	rightBox := gtk.NewBox(gtk.OrientationVertical, 0)
	rightBox.SetHExpand(true)
	rightBox.Append(authorTsBox)
	rightBox.Append(content)

	bigBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	bigBox.Append(&avatar.Widget)
	bigBox.Append(rightBox)

	bigBox.AddCSSClass("message-cozy")
	messageCSS(bigBox)

	msg := &cozyMessage{
		Box:    bigBox,
		ev:     ev,
		parent: parent,

		avatar:    avatar,
		sender:    nameLabel,
		timestamp: timestamp,
		content:   content,
	}

	msg.asyncFetch()
	return msg
}

func (m *cozyMessage) asyncFetch() {
	opt := mauthor.WithWidgetColor(m.sender)

	go func() {
		markup := mauthor.Markup(m.parent.Client(), m.ev.RoomID, m.ev.SenderID, opt)
		glib.IdleAdd(func() { m.sender.SetMarkup(markup) })

		mxc, _ := m.parent.Client().AvatarURL(m.ev.SenderID)
		if mxc != nil {
			setAvatar(m.parent.Context(), m.avatar, m.parent.Client(), *mxc)
		}
	}()
}

// setAvatar is safe to be called concurrently.
func setAvatar(ctx context.Context, a *adw.Avatar, client *gotktrix.Client, mxc matrix.URL) {
	avatarURL, _ := client.SquareThumbnail(mxc, avatarSize)
	imgutil.AsyncGET(ctx, avatarURL, a.SetCustomImage)
}

func (m *cozyMessage) Event() event.RoomEvent { return *m.ev }

// apiUpdate performs an asynchronous API update.
func (m *cozyMessage) apiUpdate() {
	// TODO
}
