package message

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/chanbakjsd/gotrix/event"
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

// Message describes a generic message type.
type Message interface {
	gtk.Widgetter
	// Event returns the origin room event.
	Event() event.RoomEvent
}

// MessageViewer describes the parent that holds messages.
type MessageViewer interface {
	Window() *gtk.Window
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
	var msg Message

	switch ev := ev.(type) {
	case event.RoomMessageEvent:
		if lastIsAuthor(parent, ev) {
			msg = newCollapsedMessage(parent, &ev)
		} else {
			msg = newCozyMessage(parent, &ev)
		}
	default:
		msg = newEventMessage(parent, ev)
	}

	bind(parent, msg)
	return msg
}

func lastIsAuthor(parent MessageViewer, ev event.RoomMessageEvent) bool {
	last := parent.LastMessage()
	// Ensure that the last message IS a cozy OR compact message.
	switch last := last.(type) {
	case *cozyMessage:
		return last.ev.SenderID == ev.SenderID
	case *collapsedMessage:
		return last.ev.SenderID == ev.SenderID
	default:
		return false
	}
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
		margin: 0 10px;
		color: alpha(@theme_fg_color, 0.8);
	}
`)

func newEventMessage(parent MessageViewer, ev event.RoomEvent) *eventMessage {
	action := gtk.NewLabel("")
	action.SetXAlign(0)
	action.AddCSSClass("message-event")
	action.SetWrap(true)
	action.SetWrapMode(pango.WrapWordChar)
	bindExtraMenu(action)

	author := mauthor.Markup(
		parent.Client().Offline(), ev.Room(), ev.Sender(),
		mauthor.WithWidgetColor(action),
	)

	msg := author + " "

	switch ev := ev.(type) {
	case event.RoomCreateEvent:
		msg += "created this room."
	case event.RoomMemberEvent:
		switch ev.NewState {
		case event.MemberInvited:
			msg += "was invited."
		case event.MemberJoined:
			msg += "joined."
		case event.MemberLeft:
			msg += "left."
		case event.MemberBanned:
			msg += "was banned."
		default:
			msg += escapef("performed member action %q.", ev.NewState)
		}
	case event.RoomPowerLevelsEvent:
		msg += "changed the room's permissions."
	case event.RoomJoinRulesEvent:
		switch ev.JoinRule {
		case event.JoinPublic:
			msg += "made the room public."
		case event.JoinInvite:
			msg += "made the room invite-only."
		default:
			msg += escapef("changed the join rule to %q.", ev.JoinRule)
		}
	case event.RoomHistoryVisibilityEvent:
		switch ev.Visibility {
		case event.VisibilityInvited:
			msg += "made the room's history visible to all invited members."
		case event.VisibilityJoined:
			msg += "made the room's history visible to all current members."
		case event.VisibilityShared:
			msg += "made the room's history visible to all past members."
		case event.VisibilityWorldReadable:
			msg += "made the room's history world-readable."
		default:
			msg += escapef("changed the room history visibility to %q.", ev.Visibility)
		}
	case event.RoomGuestAccessEvent:
		switch ev.GuestAccess {
		case event.GuestAccessCanJoin:
			msg += "allowed guests to join the room."
		case event.GuestAccessForbidden:
			msg += "denied guests from joining the room."
		default:
			msg += escapef("changed the room's guess access rule to %q.", ev.GuestAccess)
		}
	case event.RoomNameEvent:
		msg += "changed the room's name to <i>" + html.EscapeString(ev.Name) + "</i>."
	case event.RoomTopicEvent:
		msg += "changed the room's topic to <i>" + html.EscapeString(ev.Topic) + "</i>."
	default:
		msg += fmt.Sprintf("sent a %T event.", ev)
	}

	action.SetMarkup(msg)

	messageCSS(action)

	return &eventMessage{
		Label: action,
		ev:    ev,
	}
}

func escapef(f string, v ...interface{}) string {
	return html.EscapeString(fmt.Sprintf(f, v...))
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

func newCollapsedMessage(parent MessageViewer, ev *event.RoomMessageEvent) *collapsedMessage {
	client := parent.Client().Offline()

	timestamp := newTimestamp(ev.OriginTime, false)
	timestamp.SetSizeRequest(avatarWidth, -1)

	content := mcontent.New(client, ev)
	bindExtraMenu(content)

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

func newCozyMessage(parent MessageViewer, ev *event.RoomMessageEvent) *cozyMessage {
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
	bindExtraMenu(content)

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
