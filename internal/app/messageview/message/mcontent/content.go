package mcontent

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

const (
	maxWidth  = 300
	maxHeight = 500
)

// Content is a message content widget.
type Content struct {
	gtk.Widgetter // may be box or parts[0]

	box   *gtk.Box
	parts []contentPart
}

// New parses the given room message event and renders it into a Content widget.
func New(ctx context.Context, msg event.RoomMessageEvent) *Content {
	var parts []contentPart

	switch msg.MsgType {
	case event.RoomMessageText:
		parts = []contentPart{newTextContent(ctx, msg)}
	case event.RoomMessageVideo:
		parts = []contentPart{newVideoContent(ctx, msg)}
	case event.RoomMessageImage:
		parts = []contentPart{newImageContent(ctx, msg)}

	// case event.RoomMessageEmote:
	// case event.RoomMessageNotice:
	// case event.RoomMessageFile:
	// case event.RoomMessageAudio:
	// case event.RoomMessageLocation:
	default:
		parts = []contentPart{newUnknownContent(msg)}
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

func newUnknownContent(msg event.RoomMessageEvent) unknownContent {
	l := gtk.NewLabel("Unknown message type " + string(msg.MsgType) + ".")
	l.SetXAlign(0)
	l.SetWrap(true)
	l.SetWrapMode(pango.WrapWordChar)
	unknownContentCSS(l)
	return unknownContent{l}
}

func (c unknownContent) content() {}

type erroneousContent struct {
	*adw.StatusPage
}

func newErroneousContent(desc string, w, h int) erroneousContent {
	p := adw.NewStatusPage()
	p.SetTitle("Content Error")
	p.SetDescription(desc)
	p.SetIconName("image-missing")
	return erroneousContent{p}
}

func (c erroneousContent) content() {}
