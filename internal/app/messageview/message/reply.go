package message

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

type Reply struct {
	*gtk.Box
	ctx context.Context

	box struct {
		header  *gtk.Box
		content *gtk.Label
	}

	replyID matrix.EventID
	roomID  matrix.RoomID

	done bool
}

var replyInfoCSS = cssutil.Applier("message-reply-info", `
	.message-reply-info {
		color: alpha(@theme_fg_color, 0.85);
	}
`)

var replyContentCSS = cssutil.Applier("message-reply-content", `
	.message-reply-content {
		/* color: alpha(@theme_fg_color, 0.85); */
	}
`)

var replyCSS = cssutil.Applier("message-reply", `
	.message-reply {
		margin-bottom: 2px;
		border-left: 3px solid alpha(@theme_fg_color, 0.5);
		padding: 0 5px;
	}
`)

// NewReply creates a new Reply widget.
func NewReply(ctx context.Context, roomID matrix.RoomID, eventID matrix.EventID) *Reply {
	r := Reply{
		ctx:     ctx,
		replyID: eventID,
		roomID:  roomID,
	}

	info := gtk.NewLabel(locale.S(ctx, "In reply to "))
	replyInfoCSS(info)

	r.box.header = gtk.NewBox(gtk.OrientationHorizontal, 0)
	r.box.header.Append(info)

	r.box.content = gtk.NewLabel("")
	r.box.content.SetXAlign(0)
	r.box.content.SetEllipsize(pango.EllipsizeEnd)
	r.box.content.SetSingleLineMode(true)
	replyContentCSS(r.box.content)

	r.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	r.Box.Append(r.box.header)
	r.Box.Append(r.box.content)
	replyCSS(r.Box)

	return &r
}

// InvalidateContent invalidates the Reply's content. For now, this function
// does nothing after being called more than once.
func (r *Reply) InvalidateContent() {
	if r.done {
		return
	}
	r.done = true

	gtkutil.Async(r.ctx, func() func() {
		client := gotktrix.FromContext(r.ctx)

		ev, err := client.RoomTimelineEvent(r.roomID, r.replyID)
		if err != nil {
			return func() {
				r.useError(err)
				r.done = false
			}
		}

		return func() {
			author := mauthor.NewChip(r.ctx, r.roomID, ev.RoomInfo().Sender)
			author.Unpad()
			r.box.header.Append(author)

			message, ok := ev.(*event.RoomMessageEvent)
			if !ok {
				r.setContent(RenderEvent(r.ctx, ev))
			} else {
				// TODO: handle message.FormattedBody.
				r.setContent(message.Body)
			}
		}
	})
}

func (r *Reply) useError(err error) {
	r.setContent(markuputil.Error(err.Error()))
}

func (r *Reply) setContent(content string) {
	r.box.content.SetText(content)
	r.box.content.SetTooltipText(content)
}

/*
type replyWalker struct {
	*Reply
	state   *renderState
	reply   bool
	render  bool
	content bool
}

func (r *replyWalker) isReplyURL(url string) bool {
	return strings.Contains(url,
		fmt.Sprintf("https://matrix.to/%s/%s", r.RoomID, r.ReplyID),
	)
}

func (r *replyWalker) walkChildren(n *html.Node) traverseStatus {
	for n := n.FirstChild; n != nil; n = n.NextSibling {
		switch r.walkNode(n) {
		case traverseOK:
			// traverseChildren never returns traverseSkipChildren.
			if r.walkChildren(n) == traverseFailed {
				return traverseFailed
			}
		case traverseSkipChildren:
			continue
		case traverseFailed:
			return traverseFailed
		}
	}

	return traverseOK
}

func (r *replyWalker) walkNode(n *html.Node) traverseStatus {
	switch n.Type {
	case html.ElementNode:
		switch n.Data {
		case "mx-reply":
			r.reply = true
			r.walkChildren(n)
			r.reply = false
			return traverseSkipChildren

		case "blockquote":
			r.render = true
			r.walkChildren(n)
			r.render = false
			return traverseSkipChildren

		case "a":
			if !r.render || !r.reply || !r.isReplyURL(nodeAttr(n, "href")) {
				return traverseOK
			}
			r.useNodes(n)
			return traverseSkipChildren
		}
	}
	return traverseOK
}

func (r *replyWalker) useNodes(from *html.Node) {
	parent := *from.Parent
	parent.FirstChild = from

	w, ok := renderHTMLNode(r.ctx, r.RoomID, &parent)
	if ok {
		r.setContent(w)
	}
}
*/
