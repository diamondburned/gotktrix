package mcontent

import (
	"context"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotrix/event"
)

type contentPart interface {
	gtk.Widgetter
	content()
}

type editableContentPart interface {
	edit(body MessageBody)
}

type loadableContentPart interface {
	LoadMore()
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

func newRedactedContent(ctx context.Context, red *event.RoomRedactionEvent) redactedContent {
	image := gtk.NewImageFromIconName("edit-delete-symbolic")
	image.SetIconSize(gtk.IconSizeNormal)

	label := gtk.NewLabel("")
	label.SetYAlign(0)

	p := locale.FromContext(ctx)

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

func newUnknownContent(ctx context.Context, ev *event.RoomMessageEvent) unknownContent {
	p := locale.FromContext(ctx)
	msg := p.Sprintf("Unknown message type %q.", string(ev.MessageType))

	l := gtk.NewLabel(msg)
	l.SetXAlign(0)
	l.SetWrap(true)
	l.SetWrapMode(pango.WrapWordChar)
	unknownContentCSS(l)

	return unknownContent{l}
}

func (c unknownContent) content() {}
