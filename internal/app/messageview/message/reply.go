package message

import (
	"context"
	"fmt"
	"html"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

type Reply struct {
	*gtk.Box
	ctx  context.Context
	view MessageViewer

	box struct {
		header  *gtk.Box
		info    *gtk.Label
		content *gtk.Label
	}

	event event.RoomEvent

	replyID    matrix.EventID
	roomID     matrix.RoomID
	mentionURL string

	link bool
	done bool
}

var replyInfoCSS = cssutil.Applier("message-reply-info", `
	.message-reply-info {
		color: alpha(@theme_fg_color, 0.85);
	}
`)

var replyContentCSS = cssutil.Applier("message-reply-content", `
	.message-reply-content {
		caret-color: transparent;
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
func NewReply(ctx context.Context, v MessageViewer, roomID matrix.RoomID, eventID matrix.EventID) *Reply {
	r := Reply{
		ctx:        ctx,
		view:       v,
		replyID:    eventID,
		roomID:     roomID,
		mentionURL: mentionURL(roomID, eventID),
	}

	r.box.info = gtk.NewLabel(locale.S(ctx, "In reply to "))
	replyInfoCSS(r.box.info)

	r.box.header = gtk.NewBox(gtk.OrientationHorizontal, 0)
	r.box.header.Append(r.box.info)

	r.box.content = gtk.NewLabel("")
	r.box.content.SetXAlign(0)
	r.box.content.SetEllipsize(pango.EllipsizeEnd)
	r.box.content.SetSingleLineMode(true)
	r.box.content.SetSelectable(true)
	replyContentCSS(r.box.content)

	r.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	r.Box.Append(r.box.header)
	r.Box.Append(r.box.content)
	replyCSS(r.Box)

	return &r
}

// MentionURL returns the URL to matrix.to for the message that this is replying
// to.
func (r *Reply) MentionURL() string {
	return r.mentionURL
}

func mentionURL(roomID matrix.RoomID, replyID matrix.EventID) string {
	return fmt.Sprintf(`https://matrix.to/#/%s/%s`, roomID, replyID)
}

// ShowContent opens a new Popover with the message content.
func (r *Reply) ShowContent() {
	if r.event == nil {
		return
	}

	msg := NewCozyMessage(r.ctx, r.view, r.event, nil)
	msg.LoadMore()
	gtk.BaseWidget(msg).SetVAlign(gtk.AlignStart)

	view := gtk.NewViewport(nil, nil)
	view.SetChild(msg)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetMinContentWidth(250)
	scroll.SetMaxContentWidth(650)
	scroll.SetMinContentHeight(10)
	scroll.SetMaxContentHeight(600)
	scroll.SetPropagateNaturalWidth(true)
	scroll.SetPropagateNaturalHeight(true)
	scroll.SetChild(view)

	p := gtk.NewPopover()
	p.SetParent(r.box.info)
	p.SetPosition(gtk.PosTop)
	p.SetChild(scroll)
	p.Popup()
	p.ConnectHide(func() {
		glib.TimeoutSecondsAdd(2, p.Unparent)
	})
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
			r.event = ev
			r.bindLink()

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

func (r *Reply) bindLink() {
	if r.link {
		return
	}

	r.link = true

	r.box.info.SetMarkup(fmt.Sprintf(
		`<a href="%s">%s</a>`,
		html.EscapeString(r.mentionURL), locale.S(r.ctx, "In reply to "),
	))

	r.box.info.ConnectActivateLink(func(link string) bool {
		if link == r.mentionURL {
			if !r.view.ScrollTo(r.replyID) {
				r.ShowContent()
			}
			return true
		}
		return false
	})

}

func (r *Reply) useError(err error) {
	r.box.content.SetMarkup(textutil.ErrorMarkup(err.Error()))
}

func (r *Reply) setContent(content string) {
	r.box.content.SetText(content)
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
