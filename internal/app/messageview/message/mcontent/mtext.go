package mcontent

import (
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

type textContent struct {
	*gtk.TextView
}

var textContentCSS = cssutil.Applier("mcontent-text", `
	textview.mcontent-text,
	textview.mcontent-text text {
		background-color: transparent;
	}
`)

func newTextContent(msg event.RoomMessageEvent) textContent {
	text := gtk.NewTextView()
	text.SetCursorVisible(false)
	text.SetHExpand(true)
	text.SetEditable(false)
	text.SetWrapMode(gtk.WrapWordChar)
	textContentCSS(text)

	body := strings.Trim(msg.Body, "\n")

	buf := text.Buffer()
	buf.SetText(body, len(body))

	return textContent{
		TextView: text,
	}
}

func (c textContent) content() {}
