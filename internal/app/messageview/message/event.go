package message

import (
	"fmt"
	"html"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

type erroneousEvent struct {
	event.RoomEventInfo
	raw *event.RawEvent
	err error
}

func (e erroneousEvent) Type() event.Type {
	return e.raw.Type
}

// WrapErroneousEvent wraps the given raw event into another event that will be
// rendered as an erroneous event in the EventMessage component.
func WrapErroneousEvent(raw *event.RawEvent, err error) event.RoomEvent {
	return erroneousEvent{
		RoomEventInfo: event.RoomEventInfo{
			RoomID:     raw.RoomID,
			EventID:    raw.ID,
			SenderID:   raw.Sender,
			OriginTime: raw.OriginServerTime,
		},
		raw: raw,
		err: err,
	}
}

// eventMessage is a mini-message.
type eventMessage struct {
	*gtk.Label

	eventBox
}

var _ = cssutil.WriteCSS(`
	.message-event {
		font-size: .9em;
		margin: 0 10px;
		color: alpha(@theme_fg_color, 0.8);
	}
`)

func (v messageViewer) eventMessage(box eventBox) *eventMessage {
	action := gtk.NewLabel("")
	action.SetXAlign(0)
	action.AddCSSClass("message-event")
	action.SetWrap(true)
	action.SetWrapMode(pango.WrapWordChar)
	bindExtraMenu(action)

	author := mauthor.Markup(
		v.client().Offline(), box.raw.RoomID, box.raw.Sender,
		mauthor.WithWidgetColor(action),
	)

	msg := author + " " + EventMessageTail(v.client().Offline(), box.ev)
	action.SetMarkup(msg)

	messageCSS(action)

	return &eventMessage{
		Label:    action,
		eventBox: box,
	}
}

func escapef(f string, v ...interface{}) string {
	return html.EscapeString(fmt.Sprintf(f, v...))
}

// TODO: make EventMessageTail render the full string (-Tail)
// TODO: add Options into EventMessage

// EventMessageTail returns the markup tail of an event message. It does NOT
// support RoomMessageEvent.
func EventMessageTail(c *gotktrix.Client, ev event.Event) string {
	switch ev := ev.(type) {
	case event.RoomCreateEvent:
		return "created this room."
	case event.RoomMemberEvent:
		return memberEventTail(c, ev)
	case event.RoomPowerLevelsEvent:
		return "changed the room's permissions."
	case event.RoomJoinRulesEvent:
		switch ev.JoinRule {
		case event.JoinPublic:
			return "made the room public."
		case event.JoinInvite:
			return "made the room invite-only."
		default:
			return escapef("changed the join rule to %q.", ev.JoinRule)
		}
	case event.RoomHistoryVisibilityEvent:
		switch ev.Visibility {
		case event.VisibilityInvited:
			return "made the room's history visible to all invited members."
		case event.VisibilityJoined:
			return "made the room's history visible to all current members."
		case event.VisibilityShared:
			return "made the room's history visible to all past members."
		case event.VisibilityWorldReadable:
			return "made the room's history world-readable."
		default:
			return escapef("changed the room history visibility to %q.", ev.Visibility)
		}
	case event.RoomGuestAccessEvent:
		switch ev.GuestAccess {
		case event.GuestAccessCanJoin:
			return "allowed guests to join the room."
		case event.GuestAccessForbidden:
			return "denied guests from joining the room."
		default:
			return escapef("changed the room's guess access rule to %q.", ev.GuestAccess)
		}
	case event.RoomNameEvent:
		return "changed the room's name to <i>" + html.EscapeString(ev.Name) + "</i>."
	case event.RoomTopicEvent:
		return "changed the room's topic to <i>" + html.EscapeString(ev.Topic) + "</i>."
	case erroneousEvent:
		return fmt.Sprintf(
			`sent an erroneous event %T: <span color="red">%v</span>.`,
			ev.raw.Type, ev.err,
		)
	default:
		return fmt.Sprintf("sent a %T event.", ev)
	}
}

func memberEventTail(c *gotktrix.Client, ev event.RoomMemberEvent) string {
	// if tail := memberEventStatefulTail(c, &ev); tail != "" {
	// 	return tail
	// }

	switch ev.NewState {
	case event.MemberInvited:
		return "was invited."
	case event.MemberJoined:
		return "joined."
	case event.MemberLeft:
		return "left."
	case event.MemberBanned:
		return "banned someone."
	default:
		return escapef("performed member action %q.", ev.NewState)
	}
}

func memberEventStatefulTail(c *gotktrix.Client, past *event.RoomMemberEvent) string {
	ev, _ := c.State.RoomState(past.RoomID, past.Type(), past.StateKey())
	if ev == nil {
		return ""
	}

	current := ev.(event.RoomMemberEvent)

	// See https://matrix.org/docs/spec/client_server/r0.6.1#m-room-member.
	switch past.NewState {
	case event.MemberInvited:
		switch current.NewState {
		case event.MemberJoined:
			return "joined."
		case event.MemberLeft:
			if current.SenderID == current.UserID {
				return "rejected the invite."
			} else {
				return "had their invitation revoked."
			}
		}
	case event.MemberJoined:
		switch current.NewState {
		case event.MemberJoined:
			switch {
			case past.AvatarURL != current.AvatarURL:
				return "changed their avatar."
			case current.DisplayName != nil && *past.DisplayName != *current.DisplayName:
				return "changed their name."
			default:
				return "updated their information."
			}
		case event.MemberLeft:
			if current.SenderID == current.UserID {
				return "left the room."
			} else {
				return "was kicked."
			}
		case event.MemberBanned:
			return "was kicked and banned."
		}
	case event.MemberBanned:
		switch current.NewState {
		case event.MemberLeft:
			return "unbanned someone."
		}
	}

	return ""
}
