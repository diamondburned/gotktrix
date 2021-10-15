package mcontent

import (
	"context"
	"html"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

type contentPart interface {
	gtk.Widgetter
	content()
}

// ---

type redactedContent struct {
	*gtk.Box
}

var redactedCSS = cssutil.Applier("mcontent-redacted", `
	.mcontent-redacted {
		opacity: 0.75;
	}
`)

func newRedactedContent(ctx context.Context, red event.RoomRedactionEvent) redactedContent {
	image := gtk.NewImageFromIconName("edit-delete-symbolic")
	image.SetIconSize(gtk.IconSizeNormal)

	label := gtk.NewLabel("")
	label.SetYAlign(0)

	p := locale.Printer(ctx)

	if red.Reason != "" {
		red.Reason = strings.TrimSuffix(red.Reason, ".")
		label.SetText(p.Sprintf("[redacted, reason: %s.]", red.Reason))
	} else {
		label.SetText(p.Sprint("[redacted]"))
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.Append(image)
	box.Append(label)
	redactedCSS(box)

	return redactedContent{box}
}

func (c redactedContent) content() {}

// ---

type unknownContent struct {
	*gtk.Label
}

var unknownContentCSS = cssutil.Applier("mcontent-unknown", `
	.mcontent-unknown {
		font-size: 0.9em;
		color: alpha(@theme_fg_color, 0.8);
	}
`)

func newUnknownContent(ctx context.Context, msgBox *gotktrix.EventBox) unknownContent {
	var msg string

	p := locale.Printer(ctx)

	if msgBox.Type == event.TypeRoomMessage {
		e, _ := msgBox.Parse()
		emsg := e.(event.RoomMessageEvent)

		msg = p.Sprintf("Unknown message type %q.", string(emsg.MsgType))
	} else {
		msg = p.Sprintf("Unknown event type %q.", msgBox.Type)
	}

	l := gtk.NewLabel(msg)
	l.SetXAlign(0)
	l.SetWrap(true)
	l.SetWrapMode(pango.WrapWordChar)
	unknownContentCSS(l)
	return unknownContent{l}
}

func (c unknownContent) content() {}

// ---

type erroneousContent struct {
	*gtk.Box
}

func newErroneousContent(ctx context.Context, desc string, w, h int) erroneousContent {
	p := locale.Printer(ctx)

	l := gtk.NewLabel("")
	l.SetMarkup(p.Sprintf(
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
