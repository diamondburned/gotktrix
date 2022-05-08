package text

import (
	"context"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/md"
	"github.com/diamondburned/gotrix/matrix"
)

// RenderMetadata contains additional metadata that the message contains.
type RenderMetadata struct {
	// RefID is the ID to the message that the current message is a reply to, or
	// empty if there isn't one.
	RefID matrix.EventID
	// URLs is the list of valid URLs in the message. It is used for generating
	// rich embeds.
	URLs []string
}

// RenderWidgetter extends a Widgetter.
type RenderWidgetter interface {
	gtk.Widgetter
	SetExtraMenu(model gio.MenuModeller)
}

// RenderWidget describes the output of the render including the widget that
// contains the rendered information.
type RenderWidget struct {
	RenderWidgetter
	RenderMetadata
}

var plainTextCSS = cssutil.Applier("mcontent-plain-text", `
	.mcontent-plain-text {
		caret-color: transparent;
		color: @theme_fg_color;
	}
`)

// RenderText renders the given plain text.
func RenderText(ctx context.Context, text string) RenderWidget {
	text = strings.Trim(text, "\n")

	body := gtk.NewLabel("")
	body.SetSelectable(true)
	body.SetWrap(true)
	body.SetWrapMode(pango.WrapWordChar)
	body.SetXAlign(0)
	plainTextCSS(body)

	var meta RenderMetadata

	if md.IsUnicodeEmoji(text) {
		body.SetAttributes(md.EmojiAttrs)
		body.SetText(text)
	} else {
		if html, urls := hyperlink(html.EscapeString(text)); len(urls) > 0 {
			meta.URLs = urls
			body.SetMarkup(html)
		} else {
			body.SetText(text)
		}
	}

	// Annoying workaround to prevent the whole label from being selected when
	// initially focused.
	gtkutil.OnFirstDraw(body, func() { body.SelectRegion(0, 0) })

	return RenderWidget{
		RenderWidgetter: body,
		RenderMetadata:  meta,
	}
}
