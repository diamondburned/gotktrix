package message

import (
	"context"
	"encoding/json"
	"fmt"
	"html"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/sys"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

// eventMessage is a mini-message.
type eventMessage struct {
	*gtk.Label
	parent messageViewer
}

var _ = cssutil.WriteCSS(`
	.message-event {
		color: alpha(@theme_fg_color, 0.8);
		font-size: 0.9em;
	}
`)

func (v messageViewer) eventMessage() *eventMessage {
	action := gtk.NewLabel("")
	action.SetXAlign(0)
	action.AddCSSClass("message-event")
	action.SetWrap(true)
	action.SetWrapMode(pango.WrapWordChar)
	action.SetMarginStart(avatarWidth)

	action.SetMarkup(RenderEvent(v.Context, v.event))

	messageCSS(action)
	bindParent(v, action, action)

	return &eventMessage{
		Label:  action,
		parent: v,
	}
}

func (m *eventMessage) Event() event.RoomEvent {
	return m.parent.event
}

func (m *eventMessage) SetBlur(blur bool) {
	m.SetSensitive(!blur)
	setBlurClass(m, blur)
}

func (m *eventMessage) OnRelatedEvent(ev event.RoomEvent) bool {
	return false
}

func (m *eventMessage) LoadMore() {}

// TODO: make EventMessageTail render the full string (-Tail)
// TODO: add Options into EventMessage

type eventRenderer struct {
	ctx    context.Context
	client *gotktrix.Client
	roomEv *event.RoomEventInfo
}

func (r eventRenderer) author(uID matrix.UserID, mods ...mauthor.MarkupMod) string {
	mods = append(mods, mauthor.WithMinimal(), nil)
	gtkutil.InvokeMain(func() {
		mods[len(mods)-1] = mauthor.WithWidgetColor()
	})

	return mauthor.Markup(r.client, r.roomEv.RoomID, uID, mods...)
}

func (r eventRenderer) sender() string {
	return r.author(r.roomEv.Sender)
}

func (r eventRenderer) displayName(ev *event.RoomMemberEvent) string {
	if ev.DisplayName == nil || *ev.DisplayName == "" {
		return r.author(ev.UserID)
	}
	return r.author(ev.UserID, mauthor.WithName(*ev.DisplayName))
}

// RenderEvent returns the markup tail of an event message.
func RenderEvent(ctx context.Context, ev event.RoomEvent) string {
	r := eventRenderer{
		ctx:    ctx,
		client: gotktrix.FromContext(ctx).Offline(),
		roomEv: ev.RoomInfo(),
	}

	p := locale.FromContext(ctx)

	if redaction := r.roomEv.Unsigned.RedactReason; redaction != nil {
		return fmt.Sprintf(
			`%s: <span alpha="80%%"><i>%s</i></span>`,
			r.sender(), locale.S(ctx, "message redacted."),
		)
	}

	// Treat the RoomMemberEvent specially, because it has a UserID field that
	// may not always match the SenderID, especially if it's banning.
	if member, ok := ev.(*event.RoomMemberEvent); ok {
		// TODO: re-render author to be UserID instead.
		return memberEvent(r, member)
	}

	switch ev := ev.(type) {
	case *event.RoomMessageEvent:
		// TODO: light HTML renderer to Pango markup.
		return fmt.Sprintf(`%s: <span alpha="80%%">%s</span>`, r.sender(), html.EscapeString(ev.Body))
	case *event.RoomCreateEvent:
		return p.Sprintf("%s created this room.", r.sender())
	case *event.RoomPowerLevelsEvent:
		return p.Sprintf("%s changed the room's permissions.", r.sender())
	case *event.RoomJoinRulesEvent:
		switch ev.JoinRule {
		case event.JoinPublic:
			return p.Sprintf("%s made the room public.", r.sender())
		case event.JoinInvite:
			return p.Sprintf("%s made the room invite-only.", r.sender())
		default:
			return p.Sprintf("%s changed the join rule to %q.", r.sender(), ev.JoinRule)
		}
	case *event.RoomHistoryVisibilityEvent:
		switch ev.Visibility {
		case event.VisibilityInvited:
			return p.Sprintf("%s made the room's history visible to all invited members.", r.sender())
		case event.VisibilityJoined:
			return p.Sprintf("%s made the room's history visible to all current members.", r.sender())
		case event.VisibilityShared:
			return p.Sprintf("%s made the room's history visible to all past members.", r.sender())
		case event.VisibilityWorldReadable:
			return p.Sprintf("%s made the room's history world-readable.", r.sender())
		default:
			return p.Sprintf("%s changed the room history visibility to %q.", r.sender(), ev.Visibility)
		}
	case *event.RoomGuestAccessEvent:
		switch ev.GuestAccess {
		case event.GuestAccessCanJoin:
			return p.Sprintf("%s allowed guests to join the room.", r.sender())
		case event.GuestAccessForbidden:
			return p.Sprintf("%s denied guests from joining the room.", r.sender())
		default:
			return p.Sprintf("%s changed the room's guess access rule to %q.", r.sender(), ev.GuestAccess)
		}
	case *event.RoomNameEvent:
		return p.Sprintf("%s changed the room's name to <i>%s</i>.", r.sender(), html.EscapeString(ev.Name))
	case *event.RoomTopicEvent:
		return p.Sprintf("%s changed the room's topic to <i>%s</i>.", r.sender(), html.EscapeString(ev.Topic))
	case *sys.ErroneousEvent:
		return p.Sprintf(
			`%s sent an unusual event: <span color="red">%v</span>.`,
			r.sender(), ev.Err, // error already has event name
		)
	default:
		return p.Sprintf("%s sent an unhandled %s event.", r.sender(), ev.Info().Type)
	}
}

var emptyRoomMember = &event.RoomMemberEvent{}

func memberEvent(r eventRenderer, ev *event.RoomMemberEvent) string {
	p := locale.FromContext(r.ctx)

	past := pastMemberEvent(ev)
	if past == nil {
		// We can read zero values inside the struct.
		past = emptyRoomMember
	}

	// See https://matrix.org/docs/spec/client_server/r0.6.1#m-room-member.
	switch past.NewState {
	case event.MemberInvited:
		switch ev.NewState {
		case event.MemberJoined:
			return p.Sprintf("%s joined.", r.displayName(ev))
		case event.MemberLeft:
			if ev.Sender == ev.UserID {
				return p.Sprintf("%s rejected the invite.", r.displayName(ev))
			} else {
				return p.Sprintf("%s had their invitation revoked.", r.displayName(ev))
			}
		}
	case event.MemberJoined:
		switch ev.NewState {
		case event.MemberJoined:
			switch {
			case past.AvatarURL != ev.AvatarURL:
				return p.Sprintf("%s changed their avatar.", r.displayName(ev))
			case !sameDisplayName(past.DisplayName, ev.DisplayName):
				past := r.displayName(past)
				name := r.displayName(ev)
				if past == name {
					return p.Sprintf("%s changed their name.", name)
				}
				return p.Sprintf("%s changed their name to %s.", past, name)
			default:
				return p.Sprintf("%s updated their information.", r.displayName(ev))
			}
		case event.MemberLeft:
			if ev.Sender == ev.UserID {
				return p.Sprintf("%s left.", r.displayName(ev))
			} else {
				return p.Sprintf("%s was kicked.", r.displayName(ev))
			}
		case event.MemberBanned:
			return p.Sprintf("%s was kicked and banned.", r.displayName(ev))
		}
	case event.MemberBanned:
		switch ev.NewState {
		case event.MemberLeft:
			return p.Sprintf("%s was unbanned.", r.displayName(ev))
		}
	}

	return basicMemberEventTail(r, p, ev)
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

func basicMemberEventTail(r eventRenderer, p *locale.Printer, ev *event.RoomMemberEvent) string {
	switch ev.NewState {
	case event.MemberInvited:
		return p.Sprintf("%s was invited.", r.displayName(ev))
	case event.MemberJoined:
		return p.Sprintf("%s joined.", r.displayName(ev))
	case event.MemberLeft:
		return p.Sprintf("%s left.", r.displayName(ev))
	case event.MemberBanned:
		return p.Sprintf("%s was banned by %s.", r.displayName(ev), r.sender())
	default:
		return p.Sprintf("%s performed member action %q.", r.sender(), ev.NewState)
	}
}

func pastMemberEvent(ev *event.RoomMemberEvent) *event.RoomMemberEvent {
	partial := event.Partial{
		StateEventInfo: ev.StateEventInfo,
		Content:        ev.RoomEventInfo.Unsigned.PrevContent,
	}
	if partial.Content == nil {
		return nil
	}

	b, err := json.Marshal(partial)
	if err != nil {
		return nil
	}

	e, err := sys.ParseAs(b, event.TypeRoomMember)
	if err != nil {
		return nil
	}

	past := e.(*event.RoomMemberEvent)
	past.RoomID = ev.RoomID

	return past
}
