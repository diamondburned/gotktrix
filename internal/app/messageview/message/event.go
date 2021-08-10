package message

import (
	"fmt"
	"html"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

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

func (v messageViewer) EventMessage(ev event.RoomEvent) *eventMessage {
	action := gtk.NewLabel("")
	action.SetXAlign(0)
	action.AddCSSClass("message-event")
	action.SetWrap(true)
	action.SetWrapMode(pango.WrapWordChar)
	bindExtraMenu(action)

	author := mauthor.Markup(
		v.client().Offline(), ev.Room(), ev.Sender(),
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
