package mcontent

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

type emoteContent struct {
	*gtk.Label
}

var emoteContentCSS = cssutil.Applier("mcontent-emote", `
	.mcontent-emote {
		font-style: italic;
		color: alpha(@theme_fg_color, 0.9);
	}
`)

func newEmoteContent(ctx context.Context, msg *event.RoomMessageEvent) emoteContent {
	author := mauthor.Name(gotktrix.FromContext(ctx), msg.RoomID, msg.Sender)

	l := gtk.NewLabel(author + locale.S(ctx, " ") + msg.Body)
	l.SetXAlign(0)
	l.SetWrap(true)
	l.SetWrapMode(pango.WrapWordChar)
	emoteContentCSS(l)

	return emoteContent{l}
}

func (c emoteContent) content() {}
