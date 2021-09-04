package mcontent

import (
	"context"
	"fmt"
	"html"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
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
func New(ctx context.Context, msgBox *gotktrix.EventBox) *Content {
	e, err := msgBox.Parse()
	if err != nil || e.Type() != event.TypeRoomMessage {
		return wrapParts(newUnknownContent(msgBox))
	}

	msg, ok := e.(event.RoomMessageEvent)
	if !ok {
		return wrapParts(newUnknownContent(msgBox))
	}

	switch msg.MsgType {
	case event.RoomMessageNotice:
		fallthrough // treat the same as m.text
	case event.RoomMessageText:
		return wrapParts(newTextContent(ctx, msgBox))
	case event.RoomMessageVideo:
		return wrapParts(newVideoContent(ctx, msg))
	case event.RoomMessageImage:
		return wrapParts(newImageContent(ctx, msg))

	// case event.RoomMessageEmote:
	// case event.RoomMessageNotice:
	// case event.RoomMessageFile:
	// case event.RoomMessageAudio:
	// case event.RoomMessageLocation:
	default:
		return wrapParts(newUnknownContent(msgBox))
	}
}

func wrapParts(parts ...contentPart) *Content {
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

func newUnknownContent(msgBox *gotktrix.EventBox) unknownContent {
	var msg string

	if msgBox.Type == event.TypeRoomMessage {
		e, _ := msgBox.Parse()
		emsg := e.(event.RoomMessageEvent)

		msg = fmt.Sprintf("Unknown message type %s.", string(emsg.MsgType))
	} else {
		msg = fmt.Sprintf("Unknown event type %s.", msgBox.Type)
	}

	l := gtk.NewLabel(msg)
	l.SetXAlign(0)
	l.SetWrap(true)
	l.SetWrapMode(pango.WrapWordChar)
	unknownContentCSS(l)
	return unknownContent{l}
}

func (c unknownContent) content() {}

type erroneousContent struct {
	*gtk.Box
}

func newErroneousContent(desc string, w, h int) erroneousContent {
	l := gtk.NewLabel("")
	l.SetMarkup(fmt.Sprintf(
		`<span color="red">Content error:</span> %s`,
		html.EscapeString(desc),
	))

	img := gtk.NewImageFromIconName("image-missing-symbolic")
	img.SetIconSize(gtk.IconSizeNormal)

	b := gtk.NewBox(gtk.OrientationHorizontal, 2)
	b.Append(img)
	b.Append(l)

	return erroneousContent{b}
}

func (c erroneousContent) content() {}
