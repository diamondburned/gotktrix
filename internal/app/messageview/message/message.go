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

// newNameLabel creates a new Label from the given name.
func newNameLabel(c *gotktrix.Client, name gotktrix.MemberName) *gtk.Label {
	l := gtk.NewLabel("")
	setNameLabel(l, c, name)

	return l
}

// setNameLabel updates the given label to show the given member name.
func setNameLabel(l *gtk.Label, c *gotktrix.Client, name gotktrix.MemberName) {
	// TODO: pronouns
	// TODO: colors
	// TODO: maybe bridge role colors?

	if !name.Ambiguous {
		l.SetLabel(name.Name)
		return
	}

	l.SetMarkup(fmt.Sprintf(
		`%s <span fgalpha="85%%" scale="0.85">(%s)</span>`,
		html.EscapeString(name.Name), html.EscapeString(string(name.ID)),
	))
}

const (
	avatarSize  = 36
	avatarWidth = 36 + 8*2 // keep consistent with CSS
)

var messageCSS = cssutil.Applier("message-message", `
	/* .message-collapsed */
	/* .message-cozy */
	/* .message-event */
`)

// eventMessage is a mini-message.
type eventMessage struct {
	*gtk.Box
	ev event.RoomEvent

	sender *gtk.Label
	action *gtk.Label
}

var _ = cssutil.WriteCSS(`
	.message-event {
		font-size: .9em;
		color: alpha(@theme_fg_color, 0.8);
	}
`)

func newEventMessage(parent MessageViewer, ev event.RoomEvent) Message {
	client := parent.Client().Offline()

	name, _ := client.MemberName(ev.Room(), ev.Sender())
	nameLabel := newNameLabel(client, name)

	action := gtk.NewLabel(fmt.Sprintf(
		"did an event %T (%s).",
		ev, ev.ID(),
	))

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.Append(nameLabel)
	box.Append(action)

	box.AddCSSClass("message-event")
	messageCSS(box)

	return &eventMessage{
		Box:    box,
		ev:     ev,
		sender: nameLabel,
		action: action,
	}
}

func (m *eventMessage) Event() event.RoomEvent { return m.ev }

var _ = cssutil.WriteCSS(`
	.message-timestamp {
		font-size: .8em;
		color: alpha(@theme_fg_color, 0.6);
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
		margin: 0px 6px;
		margin-top: 2px;
	}
	.message-cozy >box {
		margin-left: 8px;
	}
	.message-cozy .message-timestamp {
		margin-left: .5em;
	}
`)

func newCozyMessage(parent MessageViewer, ev *event.RoomMessageEvent) Message {
	var refetch bool
	client := parent.Client().Offline()

	name, err := client.MemberName(ev.Room(), ev.Sender())
	if err != nil {
		refetch = true
	}

	nameLabel := newNameLabel(client, name)

	timestamp := newTimestamp(ev.OriginTime, true)
	timestamp.SetYAlign(0)

	avatar := adw.NewAvatar(avatarSize, name.Name, true)
	avatar.SetVAlign(gtk.AlignStart)
	avatar.SetMarginStart(2)

	mxc, err := client.AvatarURL(ev.SenderID)
	if err != nil {
		refetch = true
	} else if mxc != nil {
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

	if refetch {
		msg.asyncFetch()
	}

	return msg
}

func (m *cozyMessage) asyncFetch() {
	go func() {
		name, err := m.parent.Client().MemberName(m.ev.RoomID, m.ev.SenderID)
		if err == nil {
			glib.IdleAdd(func() { setNameLabel(m.sender, m.parent.Client(), name) })
		}

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
