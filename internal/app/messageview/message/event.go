package message

import (
	"context"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// TODO: deprecate this.

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
		font-size:    .9em;
		margin-right: 10px;
		color: alpha(@theme_fg_color, 0.8);
	}
`)

func (v messageViewer) eventMessage(box eventBox) *eventMessage {
	action := gtk.NewLabel("")
	action.SetXAlign(0)
	action.AddCSSClass("message-event")
	action.SetWrap(true)
	action.SetWrapMode(pango.WrapWordChar)
	action.SetMarginStart(avatarWidth)
	bindExtraMenu(action)

	action.SetMarkup(RenderEvent(v.Context, box.raw))

	messageCSS(action)

	return &eventMessage{
		Label:    action,
		eventBox: box,
	}
}

func fescapef(w io.Writer, f string, v ...interface{}) {
	io.WriteString(w, html.EscapeString(fmt.Sprintf(f, v...)))
}

// TODO: make EventMessageTail render the full string (-Tail)
// TODO: add Options into EventMessage

// RenderEvent returns the markup tail of an event message.
func RenderEvent(ctx context.Context, raw *event.RawEvent) string {
	client := gotktrix.FromContext(ctx).Offline()
	author := func(uID matrix.UserID) string {
		window := app.FromContext(ctx).Window()
		return mauthor.Markup(
			client, raw.RoomID, raw.Sender,
			mauthor.WithWidgetColor(&window.Widget),
			mauthor.WithMinimal(),
		)
	}

	m := strings.Builder{}
	m.Grow(512)

	e, err := raw.Parse()
	if err != nil {
		m.WriteString(author(raw.Sender))
		m.WriteString(fmt.Sprintf(
			` sent an unusual event %s: <span color="red">%v</span>.`,
			raw.Type, err,
		))
		return m.String()
	}

	// Treat the RoomMemberEvent specially, because it has a UserID field that
	// may not always match the SenderID, especially if it's banning.
	if ev, ok := e.(event.RoomMemberEvent); ok {
		m.WriteString(author(ev.UserID))
		m.WriteByte(' ')
		m.WriteString(memberEventTail(raw, ev))
		return m.String()
	}

	// For other events, we can use the SenderID as the display name.
	m.WriteString(author(raw.Sender))

	if _, ok := e.(event.RoomMessageEvent); ok {
		m.WriteByte(':')
		m.WriteByte(' ')
	} else {
		m.WriteByte(' ')
	}

	switch ev := e.(type) {
	case event.RoomMessageEvent:
		m.WriteString(`<span alpha="80%">`)
		m.WriteString(html.EscapeString(ev.Body))
		m.WriteString(`</span>`)
	case event.RoomCreateEvent:
		m.WriteString("created this room.")
	case event.RoomPowerLevelsEvent:
		m.WriteString("changed the room's permissions.")
	case event.RoomJoinRulesEvent:
		switch ev.JoinRule {
		case event.JoinPublic:
			m.WriteString("made the room public.")
		case event.JoinInvite:
			m.WriteString("made the room invite-only.")
		default:
			fescapef(&m, "changed the join rule to %q.", ev.JoinRule)
		}
	case event.RoomHistoryVisibilityEvent:
		switch ev.Visibility {
		case event.VisibilityInvited:
			m.WriteString("made the room's history visible to all invited members.")
		case event.VisibilityJoined:
			m.WriteString("made the room's history visible to all current members.")
		case event.VisibilityShared:
			m.WriteString("made the room's history visible to all past members.")
		case event.VisibilityWorldReadable:
			m.WriteString("made the room's history world-readable.")
		default:
			fescapef(&m, "changed the room history visibility to %q.", ev.Visibility)
		}
	case event.RoomGuestAccessEvent:
		switch ev.GuestAccess {
		case event.GuestAccessCanJoin:
			m.WriteString("allowed guests to join the room.")
		case event.GuestAccessForbidden:
			m.WriteString("denied guests from joining the room.")
		default:
			fescapef(&m, "changed the room's guess access rule to %q.", ev.GuestAccess)
		}
	case event.RoomNameEvent:
		m.WriteString("changed the room's name to <i>" + html.EscapeString(ev.Name) + "</i>.")
	case event.RoomTopicEvent:
		m.WriteString("changed the room's topic to <i>" + html.EscapeString(ev.Topic) + "</i>.")
	default:
		m.WriteString(fmt.Sprintf("sent a %T event.", ev))
	}

	return m.String()
}

func memberEventTail(raw *event.RawEvent, ev event.RoomMemberEvent) string {
	prev := event.RawEvent{Type: raw.Type}

	switch {
	case raw.Unsigned.PrevContent != nil:
		prev.Content = raw.Unsigned.PrevContent
	case raw.PrevContent != nil:
		prev.Content = raw.PrevContent
	default:
		return basicMemberEventTail(ev)
	}

	p, err := prev.Parse()
	if err != nil {
		return basicMemberEventTail(ev)
	}

	past, ok := p.(event.RoomMemberEvent)
	if !ok {
		return basicMemberEventTail(ev)
	}

	// See https://matrix.org/docs/spec/client_server/r0.6.1#m-room-member.
	switch past.NewState {
	case event.MemberInvited:
		switch ev.NewState {
		case event.MemberJoined:
			return "joined."
		case event.MemberLeft:
			if ev.SenderID == ev.UserID {
				return "rejected the invite."
			} else {
				return "had their invitation revoked."
			}
		}
	case event.MemberJoined:
		switch ev.NewState {
		case event.MemberJoined:
			switch {
			case past.AvatarURL != ev.AvatarURL:
				return "changed their avatar."
			case !sameDisplayName(past.DisplayName, ev.DisplayName):
				return "changed their name."
			default:
				return "updated their information."
			}
		case event.MemberLeft:
			if ev.SenderID == ev.UserID {
				return "left the room."
			} else {
				return "was kicked."
			}
		case event.MemberBanned:
			return "was kicked and banned."
		}
	case event.MemberBanned:
		switch ev.NewState {
		case event.MemberLeft:
			return "was unbanned."
		}
	}

	return basicMemberEventTail(ev)
}

func sameDisplayName(n1, n2 *string) bool {
	if n1 == nil {
		return n2 == nil
	}
	if n2 == nil {
		return n1 == nil
	}
	return *n1 == *n2
}

func basicMemberEventTail(ev event.RoomMemberEvent) string {
	switch ev.NewState {
	case event.MemberInvited:
		return "was invited."
	case event.MemberJoined:
		return "joined."
	case event.MemberLeft:
		return "left."
	case event.MemberBanned:
		return "was banned."
	default:
		return fmt.Sprintf("performed member action %q.", ev.NewState)
	}
}
