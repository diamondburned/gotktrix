package message

import (
	"context"
	"encoding/json"
	"fmt"
	"html"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/sys"
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

func (m *eventMessage) OnRelatedEvent(ev event.RoomEvent) {}

func (m *eventMessage) LoadMore() {}

// TODO: make EventMessageTail render the full string (-Tail)
// TODO: add Options into EventMessage

type eventRenderer struct {
	ctx    context.Context
	client *gotktrix.Client
	window *gtk.Window
	roomEv *event.RoomEventInfo
}

func (r eventRenderer) author(uID matrix.UserID) string {
	return mauthor.Markup(r.client, r.roomEv.RoomID, uID,
		mauthor.WithWidgetColor(r.window),
		mauthor.WithMinimal(),
	)
}

func (r eventRenderer) sender() string {
	return r.author(r.roomEv.Sender)
}

// RenderEvent returns the markup tail of an event message.
func RenderEvent(ctx context.Context, ev event.RoomEvent) string {
	r := eventRenderer{
		ctx:    ctx,
		client: gotktrix.FromContext(ctx).Offline(),
		window: app.FromContext(ctx).Window(),
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
			r.sender(), ev.Err,
		)
	default:
		return p.Sprintf("%s sent a %T event.", r.sender(), ev)
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
			return p.Sprintf("%s joined.", r.author(ev.UserID))
		case event.MemberLeft:
			if ev.Sender == ev.UserID {
				return p.Sprintf("%s rejected the invite.", r.author(ev.UserID))
			} else {
				return p.Sprintf("%s had their invitation revoked.", r.author(ev.UserID))
			}
		}
	case event.MemberJoined:
		switch ev.NewState {
		case event.MemberJoined:
			switch {
			case past.AvatarURL != ev.AvatarURL:
				return p.Sprintf("%s changed their avatar.", r.author(ev.UserID))
			case !sameDisplayName(past.DisplayName, ev.DisplayName):
				return p.Sprintf("%s changed their name.", r.author(ev.UserID))
			default:
				return p.Sprintf("%s updated their information.", r.author(ev.UserID))
			}
		case event.MemberLeft:
			if ev.Sender == ev.UserID {
				return p.Sprintf("%s left.", r.author(ev.UserID))
			} else {
				return p.Sprintf("%s was kicked.", r.author(ev.UserID))
			}
		case event.MemberBanned:
			return p.Sprintf("%s was kicked and banned.", r.author(ev.UserID))
		}
	case event.MemberBanned:
		switch ev.NewState {
		case event.MemberLeft:
			return p.Sprintf("%s was unbanned.", r.author(ev.UserID))
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
		return p.Sprintf("%s was invited.", r.author(ev.UserID))
	case event.MemberJoined:
		return p.Sprintf("%s joined.", r.author(ev.UserID))
	case event.MemberLeft:
		return p.Sprintf("%s left.", r.author(ev.UserID))
	case event.MemberBanned:
		return p.Sprintf("%s was banned by %s.", r.author(ev.UserID), r.sender())
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
