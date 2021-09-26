package mcontent

import (
	"context"
	"encoding/json"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent/text"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/md"
)

type textContent struct {
	*gtk.TextView
	ctx context.Context
}

var textContentCSS = cssutil.Applier("mcontent-text", `
	textview.mcontent-text,
	textview.mcontent-text text {
		background-color: transparent;
	}
`)

const editedHTML = `<span alpha="75%" size="small">(edited)</span>`

func newTextContent(ctx context.Context, msgBox *gotktrix.EventBox) textContent {
	tview := gtk.NewTextView()
	tview.SetHExpand(true)
	tview.SetEditable(false)
	tview.SetAcceptsTab(false)
	tview.SetCursorVisible(false)
	tview.SetWrapMode(gtk.WrapWordChar)

	md.SetTabSize(tview)
	textContentCSS(tview)

	tview.Connect("map", func() {
		// Fixes 2 GTK bugs:
		// - TextViews are invisible sometimes.
		// - Multiline TextViews are sometimes only drawn as 1.
		glib.IdleAdd(func() {
			tview.QueueAllocate()
			tview.QueueResize()
		})
	})

	text.BindLinkHandler(tview, func(url string) {
		app.OpenURI(ctx, url)
	})

	c := textContent{
		TextView: tview,
		ctx:      ctx,
	}

	body, isEdited := msgBody(msgBox)
	c.setContent(body, isEdited)

	return c
}

func (c textContent) content() {}

func (c textContent) edit(body messageBody) {
	c.setContent(body, true)
}

func (c textContent) setContent(body messageBody, isEdited bool) {
	buf := c.TextView.Buffer()

	start, end := buf.Bounds()
	buf.Delete(&start, &end)

	switch body.Format {
	case event.FormatHTML:
		// Hit the fallback case if the HTML is just a Unicode emoji.
		if md.IsUnicodeEmoji(body.FormattedBody) {
			text.RenderText(c.ctx, c.TextView, body.FormattedBody)
			break
		}
		if !text.RenderHTML(c.ctx, c.TextView, body.FormattedBody) {
			// HTML failed; use c.TextView instead.
			text.RenderText(c.ctx, c.TextView, body.Body)
		}
	default:
		text.RenderText(c.ctx, c.TextView, body.Body)
	}

	if isEdited {
		end := buf.EndIter()

		append := editedHTML
		if buf.CharCount() > 0 {
			// Prepend a space if we already have text.
			append = " " + editedHTML
		}

		buf.InsertMarkup(&end, append, len(append))
	}
}

type messageBody struct {
	Body          string              `json:"body"`
	MsgType       event.MessageType   `json:"msgtype"`
	Format        event.MessageFormat `json:"format,omitempty"`
	FormattedBody string              `json:"formatted_body,omitempty"`
}

func msgBody(box *gotktrix.EventBox) (m messageBody, edited bool) {
	var body struct {
		messageBody
		NewContent messageBody `json:"m.new_content"`

		RelatesTo struct {
			RelType string         `json:"rel_type"`
			EventID matrix.EventID `json:"event_id"`
		} `json:"m.relates_to"`
	}

	if err := json.Unmarshal(box.Content, &body); err != nil {
		// This shouldn't happen, since we already unmarshaled above.
		return messageBody{}, false
	}

	if body.RelatesTo.RelType == "m.replace" {
		return body.NewContent, true
	}
	return body.messageBody, false
}
