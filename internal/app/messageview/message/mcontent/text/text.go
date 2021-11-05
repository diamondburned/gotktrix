package text

import (
	"context"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/md"
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

// RenderText renders the given plain text.
func RenderText(ctx context.Context, tview *gtk.TextView, text string) RenderMetadata {
	body := strings.Trim(text, "\n")
	tbuf := tview.Buffer()
	tbuf.SetText(body)

	var meta RenderMetadata

	if md.IsUnicodeEmoji(body) {
		start, end := tbuf.Bounds()
		tbuf.ApplyTag(md.TextTags.FromTable(tbuf.TagTable(), "_emoji"), start, end)
	} else {
		meta.URLs = autolink(tbuf)
	}

	return meta
}
