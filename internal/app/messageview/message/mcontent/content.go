package mcontent

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
)

const (
	maxWidth  = 250
	maxHeight = 300
)

// Content is a message content widget.
type Content struct {
	*gtk.Box
	ev  *event.RoomMessageEvent
	ctx context.Context

	part  contentPart
	react *reactionBox

	editedTime matrix.Timestamp
}

// New parses the given room message event and renders it into a Content widget.
func New(ctx context.Context, ev *event.RoomMessageEvent) *Content {
	var part contentPart

	switch ev.MessageType {
	case event.RoomMessageNotice:
		fallthrough
	case event.RoomMessageText:
		part = newTextContent(ctx, ev)
	case event.RoomMessageEmote:
		part = newEmoteContent(ctx, ev)
	case event.RoomMessageVideo:
		part = newVideoContent(ctx, ev)
	case event.RoomMessageImage:
		part = newImageContent(ctx, ev)
	case event.RoomMessageAudio:
		fallthrough
	case event.RoomMessageFile:
		part = newFileContent(ctx, ev)
	case event.RoomMessageLocation:
		part = newLocationContent(ctx, ev)
	}

	if part == nil {
		part = newUnknownContent(ctx, ev)
	}

	return wrapParts(ctx, ev, part)
}

func wrapParts(ctx context.Context, ev *event.RoomMessageEvent, part contentPart) *Content {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetHExpand(true)
	box.Append(part)

	return &Content{
		Box:  box,
		ev:   ev,
		ctx:  ctx,
		part: part,
	}
}

type extraMenuSetter interface {
	SetExtraMenu(gio.MenuModeller)
}

// SetExtraMenu sets the extra menu for the message content.
func (c *Content) SetExtraMenu(menu gio.MenuModeller) {
	s, ok := c.part.(extraMenuSetter)
	if ok {
		s.SetExtraMenu(menu)
	}
}

// EditedTimestamp returns either the Matrix timestamp if the message content
// has been edited or false if not.
func (c *Content) EditedTimestamp() (matrix.Timestamp, bool) {
	return c.editedTime, c.editedTime > 0
}

func (c *Content) OnRelatedEvent(ev event.RoomEvent) {
	if c.isRedacted() {
		return
	}

	switch ev := ev.(type) {
	case *event.RoomMessageEvent:
		if body, isEdited := MsgBody(ev); isEdited {
			if editor, ok := c.part.(editableContentPart); ok {
				editor.edit(body)
				c.editedTime = ev.OriginServerTime
			}
		}
	case *event.RoomRedactionEvent:
		if ev.Redacts == c.ev.ID {
			// Redacting this message itself.
			c.redact(ev)
			return
		}
		// TODO: if we have a proper graph data structure that keeps track of
		// relational events separately instead of keeping it nested in its
		// respective events, then we wouldn't need to do this.
		if c.react == nil || c.react.Remove(c.ctx, ev) {
			return
		}
	case *m.ReactionEvent:
		if ev.RelatesTo.RelType == "m.annotation" {
			c.ensureReactions()
			c.react.Add(c.ctx, ev)
		}
	}
}

func (c *Content) ensureReactions() {
	if c.react == nil {
		c.react = newReactionBox()
		c.react.AddCSSClass("mcontent-reactionrev")
		c.Append(c.react)
	}
}

func (c *Content) LoadMore() {
	if l, ok := c.part.(loadableContentPart); ok {
		l.LoadMore()
	}
}

func (c *Content) isRedacted() bool {
	_, ok := c.part.(redactedContent)
	return ok
}

func (c *Content) redact(red *event.RoomRedactionEvent) {
	c.Box.Remove(c.part)
	if c.react != nil {
		c.react.RemoveAll()
	}

	c.part = newRedactedContent(c.ctx, red)
	c.Box.Prepend(c.part)
}
