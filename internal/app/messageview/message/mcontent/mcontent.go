package mcontent

import (
	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// Content is a message content widget.
type Content struct {
	gtk.Widgetter // may be box or parts[0]

	box   *gtk.Box
	parts []contentPart
}

// New parses the given room message event and renders it into a Content widget.
func New(client *gotktrix.Client, msg *event.RoomMessageEvent) *Content {
	var parts []contentPart

	switch msg.MsgType {
	case event.RoomMessageText:
		parts = []contentPart{
			newTextContent(msg),
		}
	default:
		parts = []contentPart{
			newUnknownContent(msg),
		}

		// case event.RoomMessageEmote:
		// case event.RoomMessageNotice:
		// case event.RoomMessageImage:
		// case event.RoomMessageFile:
		// case event.RoomMessageAudio:
		// case event.RoomMessageLocation:
		// case event.RoomMessageVideo:
	}

	if len(parts) == 1 {
		return &Content{
			Widgetter: parts[0],
			parts:     parts,
		}
	}

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetHExpand(true)
	for _, part := range parts {
		box.Append(part)
	}

	return &Content{
		Widgetter: box,
		box:       box,
		parts:     parts,
	}
}

type extraMenuSetter interface {
	SetExtraMenu(gio.MenuModeller)
}

// SetExtraMenu sets the extra menu for the message content.
func (c *Content) SetExtraMenu(menu gio.MenuModeller) {
	for _, part := range c.parts {
		s, ok := part.(extraMenuSetter)
		if ok {
			s.SetExtraMenu(menu)
		}
	}
}

type contentPart interface {
	gtk.Widgetter
	content()
}

type textContent struct {
	*gtk.TextView
}

var textContentCSS = cssutil.Applier("mcontent-text", `
	textview.mcontent-text,
	textview.mcontent-text text {
		background-color: transparent;
	}
`)

func newTextContent(msg *event.RoomMessageEvent) textContent {
	text := gtk.NewTextView()
	text.SetCursorVisible(false)
	text.SetHExpand(true)
	text.SetEditable(false)
	text.SetWrapMode(gtk.WrapWordChar)
	textContentCSS(text)

	buf := text.Buffer()
	buf.SetText(msg.Body, len(msg.Body))

	return textContent{
		TextView: text,
	}
}

func (c textContent) content() {}

type imageContent struct {
	*gtk.Image
}

func (c imageContent) content() {}

type attachmentContent struct {
	*gtk.Box
}

func (c attachmentContent) content() {}

type unknownContent struct {
	*gtk.Label
}

var unknownContentCSS = cssutil.Applier("mcontent-unknown", `
	.mcontent-unknown {
		font-size: 0.9em;
		color: alpha(@theme_fg_color, 0.85);
	}
`)

func newUnknownContent(msg *event.RoomMessageEvent) unknownContent {
	l := gtk.NewLabel("Unknown message type " + string(msg.MsgType) + ".")
	l.SetXAlign(0)
	l.SetWrap(true)
	l.SetWrapMode(pango.WrapWordChar)
	unknownContentCSS(l)
	return unknownContent{l}
}

func (c unknownContent) content() {}
