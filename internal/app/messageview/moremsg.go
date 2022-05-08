package messageview

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/matrix"
)

type moreMessageState uint8

const (
	hideMoreMessages moreMessageState = iota
	unreadMessages
	mentionedMessages
)

func (s moreMessageState) css() string {
	switch s {
	case hideMoreMessages:
		return "messageview-moremsg-hidden"
	case unreadMessages:
		return "messageview-moremsg-unread"
	case mentionedMessages:
		return "messageview-moremsg-mentioned"
	default:
		panic("unreachable")
	}
}

// moreMessageBar is an InfoBar wrapper indicating users new messages in a
// clearer way.
type moreMessageBar struct {
	*gtk.InfoBar
	label *gtk.Label

	ctx    context.Context
	roomID matrix.RoomID
	state  moreMessageState
}

// 17 new messages
// X new unreads

var moreMessageBarCSS = cssutil.Applier(`messageview-moremsg`, `
	.messageview-moremsg {
		background-color: @theme_bg_color;
	}
	.messageview-moremsg > revealer > box {
		padding-top:    2px;
		padding-bottom: 2px;
		padding-left:  12px;
	}
	.messageview-moremsg button {
		padding-top:    0px;
		padding-bottom: 0px;
	}
`)

func newMoreMessageBar(ctx context.Context, roomID matrix.RoomID) *moreMessageBar {
	m := moreMessageBar{
		ctx:    ctx,
		roomID: roomID,
	}

	m.label = gtk.NewLabel("")
	m.label.AddCSSClass("messageview-moremsg-text")
	m.label.SetXAlign(0)
	m.label.SetHExpand(true)
	m.label.SetWrap(true)
	m.label.SetWrapMode(pango.WrapWordChar)
	m.label.SetLines(2)
	m.label.SetEllipsize(pango.EllipsizeEnd)

	m.InfoBar = gtk.NewInfoBar()
	m.InfoBar.AddChild(m.label)
	m.InfoBar.SetRevealed(false)

	moreMessageBarCSS(m)
	return &m
}

// AddButton adds a button with the given name into the end of the bar.
func (m *moreMessageBar) AddButton(label string) *gtk.Button {
	return m.InfoBar.AddButton(label, 0)
}

// Hide hides the bar. Invalidating the bar again might reveal it.
func (m *moreMessageBar) Hide() {
	m.setState(hideMoreMessages)
}

// Invalidate updates the state of the more messages bar. Both Set and SetState
// will automatically call this method if needed.
func (m *moreMessageBar) Invalidate() {
	client := gotktrix.FromContext(m.ctx).Offline()

	new, more := client.RoomCountUnread(m.roomID)
	if new == 0 {
		m.setState(hideMoreMessages)
		return
	}

	msg := locale.Plural(m.ctx, "%d new message.", "%d new messages.", new)
	if more {
		msg = "+" + msg
	}

	m.label.SetText(msg)
	// TODO: mentions
	m.setState(unreadMessages)
}

func (m *moreMessageBar) setState(state moreMessageState) {
	if m.state == state {
		return
	}

	m.RemoveCSSClass(m.state.css())
	m.state = state
	m.AddCSSClass(state.css())

	show := m.state != hideMoreMessages
	m.SetRevealed(show)
	m.SetSensitive(show)
}
