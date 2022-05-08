package mcontent

import (
	"context"
	"encoding/json"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent/text"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
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

func newTextContent(ctx context.Context, ev *event.RoomMessageEvent) *textContent {
	c := textContent{
		Box:    gtk.NewBox(gtk.OrientationVertical, 0),
		ctx:    ctx,
		roomID: ev.RoomID,
	}

	body, isEdited := MsgBody(ev)
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

	o := text.Opts{
		SkipReply: true,
	}

	switch body.Format {
	case event.FormatHTML:
		c.render = text.RenderHTML(c.ctx, body.Body, body.FormattedBody, c.roomID, o)
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

	if c.embeds != nil {
		c.embeds.Unparent()
		c.embeds = nil
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
func MsgBody(ev *event.RoomMessageEvent) (m MessageBody, edited bool) {
	type relatesTo struct {
		RelType string         `json:"rel_type"`
		EventID matrix.EventID `json:"event_id"`
	}

	unedited := MessageBody{
		Body:          ev.Body,
		MsgType:       ev.MessageType,
		Format:        ev.Format,
		FormattedBody: ev.FormattedBody,
	}

	if ev.Raw == nil {
		// No raw, so we can't get the new_content field. We can still guess if
		// the message is edited or not.
		var relates relatesTo
		json.Unmarshal(ev.RelatesTo, &relates)

		edited = relates.RelType == "m.replace"
		return unedited, edited
	}

	var body struct {
		Content struct {
			NewContent MessageBody `json:"m.new_content"`
			RelatesTo  relatesTo   `json:"m.relates_to"`
		}
	}

	if err := json.Unmarshal(ev.Raw, &body); err != nil {
		// This shouldn't happen, since we already unmarshaled above.
		return unedited, false
	}

	if body.Content.RelatesTo.RelType == "m.replace" {
		return body.Content.NewContent, true
	}

	return unedited, false
}
