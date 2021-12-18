package mcontent

import (
	"context"
	"encoding/json"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent/text"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

type textContent struct {
	*gtk.Box
	render text.RenderWidget
	embeds *gtk.Box

	ctx context.Context
}

var _ editableContentPart = (*textContent)(nil)

func newTextContent(ctx context.Context, msgBox *gotktrix.EventBox) *textContent {
	c := textContent{
		Box: gtk.NewBox(gtk.OrientationVertical, 0),
		ctx: ctx,
	}

	body, isEdited := msgBody(msgBox)
	c.setContent(body, isEdited)

	return &c
}

func (c *textContent) content() {}

func (c *textContent) SetExtraMenu(menu gio.MenuModeller) {
	gmenu := gio.NewMenu()
	gmenu.InsertSection(0, "Message", menu)
	c.render.SetExtraMenu(gmenu)
}

func (c *textContent) edit(body messageBody) {
	c.setContent(body, true)
}

func (c *textContent) setContent(body messageBody, isEdited bool) {
	if c.render.RenderWidgetter != nil {
		c.Box.Remove(c.render.RenderWidgetter)
	}
	if c.embeds != nil {
		c.Box.Remove(c.embeds)
	}

	switch body.Format {
	case event.FormatHTML:
		c.render = text.RenderHTML(c.ctx, body.Body, body.FormattedBody)
	default:
		c.render = text.RenderText(c.ctx, body.Body)
	}

	c.Box.Append(c.render)

	// TODO

	// if isEdited {
	// 	end := buf.EndIter()

	// 	edited := `<span alpha="75%" size="small">` + locale.S(c.ctx, "(edited)") + "</span>"
	// 	if buf.CharCount() > 0 {
	// 		// Prepend a space if we already have text.
	// 		edited = " " + edited
	// 	}

	// 	buf.InsertMarkup(end, edited)
	// }

	// c.invalidateAllocate()
}

var embedsCSS = cssutil.Applier("mcontent-embeds", `
	.mcontent-embeds > * {
		margin-top: 6px;
	}
`)

func (c *textContent) LoadMore() {
	if len(c.render.URLs) == 0 {
		return
	}

	c.embeds = gtk.NewBox(gtk.OrientationVertical, 0)
	c.embeds.SetHAlign(gtk.AlignStart)
	embedsCSS(c.embeds)

	c.Box.Append(c.embeds)
	// TODO: cancellation
	loadEmbeds(c.ctx, c.embeds, c.render.URLs)
}

// func (c *textContent) invalidateAllocate() {
// 	// Workaround to hopefully fix 2 GTK bugs:
// 	// - TextViews are invisible sometimes.
// 	// - Multiline TextViews are sometimes only drawn as 1.
// 	glib.TimeoutAdd(100, func() {
// 		c.text.QueueAllocate()
// 		c.text.QueueResize()
// 	})
// }

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
