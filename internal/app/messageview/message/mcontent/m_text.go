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
	roomID matrix.RoomID
	render text.RenderWidget
	embeds *gtk.Box

	menu gio.MenuModeller
	ctx  context.Context
}

var _ editableContentPart = (*textContent)(nil)

func newTextContent(ctx context.Context, msgBox *gotktrix.EventBox) *textContent {
	c := textContent{
		Box:    gtk.NewBox(gtk.OrientationVertical, 0),
		ctx:    ctx,
		roomID: msgBox.RoomID,
	}

	body, isEdited := MsgBody(msgBox)
	c.setContent(body, isEdited)

	return &c
}

func (c *textContent) content() {}

func (c *textContent) SetExtraMenu(messageMenu gio.MenuModeller) {
	menu := gio.NewMenu()
	menu.InsertSection(0, "Message", messageMenu)

	c.menu = menu
	c.render.SetExtraMenu(c.menu)
}

func (c *textContent) edit(body MessageBody) {
	c.setContent(body, true)
	c.LoadMore()
}

func (c *textContent) setContent(body MessageBody, isEdited bool) {
	if c.render.RenderWidgetter != nil {
		c.Box.Remove(c.render.RenderWidgetter)
	}
	if c.embeds != nil {
		c.Box.Remove(c.embeds)
	}

	switch body.Format {
	case event.FormatHTML:
		c.render = text.RenderHTML(c.ctx, body.Body, body.FormattedBody, c.roomID)
	default:
		c.render = text.RenderText(c.ctx, body.Body)
	}

	c.render.SetExtraMenu(c.menu)
	c.Box.Append(c.render)
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

type MessageBody struct {
	Body          string              `json:"body"`
	MsgType       event.MessageType   `json:"msgtype"`
	Format        event.MessageFormat `json:"format,omitempty"`
	FormattedBody string              `json:"formatted_body,omitempty"`
}

// MsgBody parses the message event and accounts for edited ones.
func MsgBody(box *gotktrix.EventBox) (m MessageBody, edited bool) {
	var body struct {
		MessageBody
		NewContent MessageBody `json:"m.new_content"`

		RelatesTo struct {
			RelType string         `json:"rel_type"`
			EventID matrix.EventID `json:"event_id"`
		} `json:"m.relates_to"`
	}

	if err := json.Unmarshal(box.Content, &body); err != nil {
		// This shouldn't happen, since we already unmarshaled above.
		return MessageBody{}, false
	}

	if body.RelatesTo.RelType == "m.replace" {
		return body.NewContent, true
	}
	return body.MessageBody, false
}
