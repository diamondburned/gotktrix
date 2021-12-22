package message

import (
	"context"
	"fmt"
	"html"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

// eventMessage is a mini-message.
type eventMessage struct {
	*gtk.Label
	parent messageViewer
}

var _ = cssutil.WriteCSS(`
	.message-event {
		font-size:    .9em;
		margin-right: 10px;
		color: alpha(@theme_fg_color, 0.8);
	}
`)

func (v messageViewer) eventMessage() *eventMessage {
	action := gtk.NewLabel("")
	action.SetXAlign(0)
	action.AddCSSClass("message-event")
	action.SetWrap(true)
	action.SetWrapMode(pango.WrapWordChar)
	action.SetMarginStart(avatarWidth)

	action.SetMarkup(RenderEvent(v.Context, v.raw))

	messageCSS(action)
	bindParent(v, action, action)

	return &eventMessage{
		Label:  action,
		parent: v,
	}
}

func (m *eventMessage) RawEvent() *gotktrix.EventBox {
	return m.parent.raw
}

func (m *eventMessage) SetBlur(blur bool) {
	m.SetSensitive(!blur)
	setBlurClass(m, blur)
}

func (m *eventMessage) OnRelatedEvent(ev *gotktrix.EventBox) {}

func (m *eventMessage) LoadMore() {}

// TODO: make EventMessageTail render the full string (-Tail)
// TODO: add Options into EventMessage

// RenderEvent returns the markup tail of an event message.
func RenderEvent(ctx context.Context, raw *gotktrix.EventBox) string {
	client := gotktrix.FromContext(ctx).Offline()
	window := app.FromContext(ctx).Window()

	authorForUser := func(userID matrix.UserID) string {
		return mauthor.Markup(
			client, raw.RoomID, userID,
			mauthor.WithWidgetColor(&window.Widget),
			mauthor.WithMinimal(),
		)
	}

	p := locale.FromContext(ctx)

	if redaction := raw.Unsigned.RedactReason; redaction != nil {
		redacted := p.Sprint("message redacted.")
		author := authorForUser(raw.Sender)
		return fmt.Sprintf(`%s: <span alpha="80%%"><i>%s</i></span>`, author, redacted)
	}

	e, err := raw.Parse()
	if err != nil {
		author := authorForUser(raw.Sender)
		return p.Sprintf(
			`%s sent an unusual event %s: <span color="red">%v</span>.`,
			author, raw.Type, err,
		)
	}

	var author string
	// Get the sender's ID, OR the user ID that the event acts on, if there's
	// one. All the strings below assume that. Don't use the state key if the
	// event fails to be parsed.
	if raw.StateKey != "" {
		author = authorForUser(matrix.UserID(raw.StateKey))
	} else {
		author = authorForUser(raw.Sender)
	}

	// Treat the RoomMemberEvent specially, because it has a UserID field that
	// may not always match the SenderID, especially if it's banning.
	if _, ok := e.(event.RoomMemberEvent); ok {
		// TODO: re-render author to be UserID instead.
		return memberEvent(ctx, raw, author)
	}

	switch ev := e.(type) {
	case event.RoomMessageEvent:
		return fmt.Sprintf(`%s: <span alpha="80%%">%s</span>`, author, html.EscapeString(ev.Body))
	case event.RoomCreateEvent:
		return p.Sprintf("%s created this room.", author)
	case event.RoomPowerLevelsEvent:
		return p.Sprintf("%s changed the room's permissions.", author)
	case event.RoomJoinRulesEvent:
		switch ev.JoinRule {
		case event.JoinPublic:
			return p.Sprintf("%s made the room public.", author)
		case event.JoinInvite:
			return p.Sprintf("%s made the room invite-only.", author)
		default:
			return p.Sprintf("%s changed the join rule to %q.", author, ev.JoinRule)
		}
	case event.RoomHistoryVisibilityEvent:
		switch ev.Visibility {
		case event.VisibilityInvited:
			return p.Sprintf("%s made the room's history visible to all invited members.", author)
		case event.VisibilityJoined:
			return p.Sprintf("%s made the room's history visible to all current members.", author)
		case event.VisibilityShared:
			return p.Sprintf("%s made the room's history visible to all past members.", author)
		case event.VisibilityWorldReadable:
			return p.Sprintf("%s made the room's history world-readable.", author)
		default:
			return p.Sprintf("%s changed the room history visibility to %q.", author, ev.Visibility)
		}
	case event.RoomGuestAccessEvent:
		switch ev.GuestAccess {
		case event.GuestAccessCanJoin:
			return p.Sprintf("%s allowed guests to join the room.", author)
		case event.GuestAccessForbidden:
			return p.Sprintf("%s denied guests from joining the room.", author)
		default:
			return p.Sprintf("%s changed the room's guess access rule to %q.", author, ev.GuestAccess)
		}
	case event.RoomNameEvent:
		return p.Sprintf("%s changed the room's name to <i>%s</i>.", author, html.EscapeString(ev.Name))
	case event.RoomTopicEvent:
		return p.Sprintf("%s changed the room's topic to <i>%s</i>.", author, html.EscapeString(ev.Topic))
	default:
		return p.Sprintf("%s sent a %T event.", author, ev)
	}
}

func pastMemberEvent(raw *gotktrix.EventBox) event.RoomMemberEvent {
	prev := event.RawEvent{
		Type: raw.Type,
	}

	switch {
	case raw.Unsigned.PrevContent != nil:
		prev.Content = raw.Unsigned.PrevContent
	case raw.PrevContent != nil:
		prev.Content = raw.PrevContent
	default:
		return event.RoomMemberEvent{}
	}

	p, err := prev.Parse()
	if err != nil {
		return event.RoomMemberEvent{}
	}

	past, ok := p.(event.RoomMemberEvent)
	if !ok {
		return event.RoomMemberEvent{}
	}

	return past
}

func memberEvent(ctx context.Context, raw *gotktrix.EventBox, author string) string {
	printer := locale.FromContext(ctx)

	parsed, _ := raw.Parse()
	ev := parsed.(event.RoomMemberEvent)

	past := pastMemberEvent(raw)

	// See https://matrix.org/docs/spec/client_server/r0.6.1#m-room-member.
	switch past.NewState {
	case event.MemberInvited:
		switch ev.NewState {
		case event.MemberJoined:
			return locale.Sprintf(ctx, "%s joined.", author)
		case event.MemberLeft:
			if ev.SenderID == ev.UserID {
				return locale.Sprintf(ctx, "%s rejected the invite.", author)
			} else {
				return locale.Sprintf(ctx, "%s had their invitation revoked.", author)
			}
		}
	case event.MemberJoined:
		switch ev.NewState {
		case event.MemberJoined:
			switch {
			case past.AvatarURL != ev.AvatarURL:
				return locale.Sprintf(ctx, "%s changed their avatar.", author)
			case !sameDisplayName(past.DisplayName, ev.DisplayName):
				return locale.Sprintf(ctx, "%s changed their name.", author)
			default:
				return locale.Sprintf(ctx, "%s updated their information.", author)
			}
		case event.MemberLeft:
			if ev.SenderID == ev.UserID {
				return locale.Sprintf(ctx, "%s left the room.", author)
			} else {
				return locale.Sprintf(ctx, "%s was kicked.", author)
			}
		case event.MemberBanned:
			return locale.Sprintf(ctx, "%s was kicked and banned.", author)
		}
	case event.MemberBanned:
		switch ev.NewState {
		case event.MemberLeft:
			return locale.Sprintf(ctx, "%s was unbanned.", author)
		}
	}

	return basicMemberEventTail(printer, ev, author)
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

func basicMemberEventTail(p *locale.Printer, ev event.RoomMemberEvent, author string) string {
	switch ev.NewState {
	case event.MemberInvited:
		return p.Sprintf("%s was invited.", author)
	case event.MemberJoined:
		return p.Sprintf("%s joined.", author)
	case event.MemberLeft:
		return p.Sprintf("%s left.", author)
	case event.MemberBanned:
		return p.Sprintf("%s was banned.", author)
	default:
		return p.Sprintf("%s performed member action %q.", author, ev.NewState)
	}
}
