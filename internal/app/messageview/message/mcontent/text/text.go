package text

import (
	"context"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/md"
)

// RenderText renders the given plain text.
func RenderText(ctx context.Context, tview *gtk.TextView, text string) {
	body := strings.Trim(text, "\n")
	tbuf := tview.Buffer()
	tbuf.SetText(body, len(body))

	if md.IsUnicodeEmoji(body) {
		start, end := tbuf.Bounds()
		tbuf.ApplyTag(md.TextTags.FromTable(tbuf.TagTable(), "_emoji"), &start, &end)
	} else {
		autolink(tbuf)
	}
}
