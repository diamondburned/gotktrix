package mcontent

import (
	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
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

func newEmojiContent(msg *event.RoomMessageEvent) emoteContent {
	l := gtk.NewLabel("*" + msg.Body + "*")
	l.SetXAlign(0)
	l.SetWrap(true)
	l.SetWrapMode(pango.WrapWordChar)
	emoteContentCSS(l)
	return emoteContent{l}
}

func (c emoteContent) content() {}
