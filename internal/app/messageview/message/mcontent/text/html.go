package text

import (
	"container/list"
	"context"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/md"
	"golang.org/x/net/html"
)

const (
	smallEmojiSize = 20
	largeEmojiSize = 48
)

const (
	maxWidth  = 300
	maxHeight = 350
)

const (
	eventURLPrefix   = "https://matrix.io/#/!"
	mentionURLPrefix = "https://matrix.to/#/@"
)

// RenderHTML tries rendering the HTML and falls back to using plain text if
// the HTML doesn't work.
func RenderHTML(ctx context.Context, text, html string, roomID matrix.RoomID) RenderWidget {
	// If html is text, then just render it as plain text, because using the
	// Label should yield much better performance than running it through the
	// parser.
	if (html == text && !mightBeHTML(html)) || md.IsUnicodeEmoji(html) {
		return RenderText(ctx, html)
	}

	rw, ok := renderHTML(ctx, roomID, html)
	if !ok {
		rw = RenderText(ctx, text)
	}

	return rw
}

// mightBeHTML returns whether or not text might be HTML.
func mightBeHTML(text string) bool {
	return strings.Contains(text, "<") || strings.Contains(text, ">")
}

type htmlBox struct {
	*gtk.Box
	list *list.List
}

// SetExtraMenu sets the extra menus of all internal texts.
func (b htmlBox) SetExtraMenu(model gio.MenuModeller) {
	for n := b.list.Front(); n != nil; n = n.Next() {
		switch block := n.Value.(type) {
		case *textBlock:
			block.SetExtraMenu(model)
		case *quoteBlock:
			block.SetExtraMenu(model)
		case *codeBlock:
			block.text.SetExtraMenu(model)
		}
	}
}

// renderHTML returns true if the HTML parsing and rendering is successful.
func renderHTML(ctx context.Context, roomID matrix.RoomID, htmlBody string) (RenderWidget, bool) {
	n, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		log.Println("invalid message HTML:", err)
		return RenderWidget{}, false
	}

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("mcontent-html-box")

	state := renderState{
		block: newBlockState(ctx, box),
		ctx:   ctx,
		room:  roomID,
		list:  0,
		// TODO: detect unicode emojis.
		large: !nodeHasText(n),
	}

	if state.traverseSiblings(n) == traverseFailed {
		return RenderWidget{}, false
	}

	rendered := RenderWidget{
		RenderWidgetter: htmlBox{
			Box:  box,
			list: state.block.list,
		},
	}

	if state.replyURL != "" {
		// The URL is guaranteed to have this suffix. The trimming will also
		// throw away the event prefix, so add it back.
		id := "!" + strings.TrimPrefix(state.replyURL, eventURLPrefix)
		// Scan everything up to the first slash.
		if end := strings.Index(id, "/"); end > -1 {
			id = id[:end]
		}
		rendered.RefID = matrix.EventID(id)
	}

	// Auto-link all buffers.
	for n := state.block.list.Front(); n != nil; n = n.Next() {
		var text *textBlock

		switch block := n.Value.(type) {
		case *textBlock:
			text = block
		case *quoteBlock:
			text = &block.textBlock
		default:
			continue
		}

		urls := autolink(text.buf)
		if len(urls) == 0 {
			continue
		}

		text.hasLink()
		rendered.URLs = append(rendered.URLs, urls...)
	}

	return rendered, true
}

type traverseStatus uint8

const (
	traverseOK traverseStatus = iota
	traverseSkipChildren
	traverseFailed
)

type renderState struct {
	block currentBlockState

	ctx  context.Context
	room matrix.RoomID

	replyURL string

	list  int
	reply bool
	pre   bool
	large bool
}

func (s *renderState) renderNode(n *html.Node) traverseStatus {
	switch n.Type {
	case html.TextNode:
		text := s.block.text()
		trimmed := trimNewLines(n.Data)

		// Make up the left-hand-side new lines.
		text.insertNewLines(trimmed.left - text.nTrailingNewLine())

		if trimmed.text == "" {
			// Ignore this segment entirely and don't write the right-trailing
			// new lines.
			return traverseOK
		}

		// Insert the trimmed string.
		text.buf.Insert(text.iter, trimmed.text)

		if nextNode := nodeNextSibling(n); nextNode != nil {
			// Only make up new lines if we still have nodes.
			text.insertNewLines(trimmed.right)
		}

		return traverseOK

	case html.ElementNode:
		switch n.Data {
		case "html", "body", "head":
			return traverseOK

		// Inline.
		case "font", "span": // data-mx-bg-color, data-mx-color
			tag := markuputil.HashTag(s.block.table, markuputil.TextTag{
				"foreground": nodeAttr(n, "data-mx-color", "color"),
				"background": nodeAttr(n, "data-mx-bg-color"),
			})
			s.renderChildrenTagged(n, tag)
			return traverseSkipChildren

		// Inline.
		case "h1", "h2", "h3", "h4", "h5", "h6",
			"em", "i", "strong", "b", "u", "strike", "del", "sup", "sub", "caption":
			s.renderChildren(n)
			return traverseSkipChildren

		// Inline.
		case "code":
			switch block := s.block.current().(type) {
			case *codeBlock:
				lang := strings.TrimPrefix(nodeAttr(n, "class"), "language-")
				block.withHighlight(lang, func(text *textBlock) {
					s.traverseChildren(n)
				})
			default:
				s.renderChildren(n)
			}

			return traverseSkipChildren

		// Block Elements.
		case "blockquote":
			s.block.quote()
			s.traverseChildren(n)
			s.block.finalizeBlock()
			return traverseSkipChildren

		// Block Elements.
		case "pre":
			s.block.code()
			s.traverseChildren(n)
			s.block.finalizeBlock()
			return traverseSkipChildren

		case "p", "div":
			// Only start and stop a new block if we're not already in a
			// blockquote, since we're not nesting anything, so doing this will
			// mess up the blockquote.
			if _, ok := s.block.current().(*quoteBlock); ok {
				s.endLine(n, 1)
			} else {
				s.block.paragraph()
				defer s.block.finalizeBlock()
			}
			s.traverseChildren(n)
			return traverseSkipChildren

		// Inline.
		case "a":
			text := s.block.richText()
			text.hasLink()

			href := nodeAttr(n, "href")
			if unescaped, err := url.PathUnescape(href); err == nil {
				// Unescape the URL if it is escaped.
				href = unescaped
			}

			if s.reply && s.replyURL == "" && strings.HasPrefix(href, eventURLPrefix) {
				// TODO: check that the inner text says "in reply to", but
				// that's probably a bad idea.
				s.replyURL = href
			}

			// -1 means don't link
			start := -1
			color := false

			switch {
			// See if this is a user mention. If yes, then write our own texts.
			case strings.HasPrefix(href, mentionURLPrefix):
				// Make the mention a link as well.
				start = text.iter.Offset()
				// Format the user ID; the trimming will trim the at symbol so
				// add it back.
				uID := matrix.UserID("@" + strings.TrimPrefix(href, mentionURLPrefix))
				mauthor.Text(gotktrix.FromContext(s.ctx), text.iter, s.room, uID,
					mauthor.WithWidgetColor(text),
					mauthor.WithMention(),
					mauthor.WithMinimal(),
					mauthor.WithShade(),
				)

			// Only bother with adding a link tag if we know that the URL
			// has a safe scheme.
			case urlIsSafe(href):
				start = text.iter.Offset()
				color = true
				s.traverseChildren(n)
			}

			if start != -1 {
				startIter := text.buf.IterAtOffset(start)
				end := text.iter.Offset()

				tag := text.emptyTag(embeddedURLPrefix + embedURL(start, end, href))
				text.buf.ApplyTag(tag, startIter, text.iter)

				if color {
					a := markuputil.LinkTags().FromTable(text.table, "a")
					text.buf.ApplyTag(a, startIter, text.iter)
				}
			}

			return traverseSkipChildren

		case "ol": // start
			s.list = 1
			s.traverseChildren(n)
			s.list = 0
			return traverseSkipChildren

		case "ul":
			s.list = 0
			// No need to reset s.list.
			return traverseOK

		case "li":
			bullet := "● "
			if s.list != 0 {
				bullet = strconv.Itoa(s.list) + ". "
				s.list++
			}

			// TODO: make this its own widget somehow.
			text := s.block.richText()
			text.buf.Insert(text.iter, "    "+bullet)

			s.renderChildren(n)
			return traverseSkipChildren

		case "hr":
			s.block.separator()
			return traverseOK
		case "br":
			s.endLine(n, 1)
			return traverseOK

		case "img": // width, height, alt, title, src(mxc)
			src := matrix.URL(nodeAttr(n, "src"))

			u, err := url.Parse(string(src))
			if err != nil || u.Scheme != "mxc" {
				// Ignore the image entirely.
				text := s.block.richText()
				text.buf.InsertMarkup(text.iter, `<span fgalpha="50%">[image]</span>`)
				return traverseOK
			}

			// TODO: figure out a way to insert a nice text representation of an
			// emoji that's invisible, so it's clipboard-and-TTS friendly. This
			// way doesn't work.
			// s.insertInvisible(nodeAttr(n, "title"))

			// TODO: consider if it's a better idea to only allow emoticons to
			// be inlined. As far as I know, nothing except emojis are really
			// good for being inlined, but that might not cover everything.
			var w, h int
			if nodeHasAttr(n, "data-mx-emoticon") {
				// If this image is a custom emoji, then we can make it big.
				if s.large {
					w, h = largeEmojiSize, largeEmojiSize
				} else {
					w, h = smallEmojiSize, smallEmojiSize
				}
			} else {
				w, h = gotktrix.MaxSize(
					parseIntOr(nodeAttr(n, "width"), maxWidth),
					parseIntOr(nodeAttr(n, "height"), maxHeight),
					maxWidth,
					maxHeight,
				)
			}

			thumbnail, _ := gotktrix.FromContext(s.ctx).Offline().ScaledThumbnail(src, w, h, 1)

			text := s.block.richText()
			md.AsyncInsertImage(s.ctx, text.iter, thumbnail, w, h)
			return traverseOK

		case "mx-reply":
			s.reply = true
			s.traverseChildren(n)
			s.reply = false
			return traverseSkipChildren

		default:
			log.Println("unknown tag", n.Data)
			s.traverseChildren(n)
			return traverseSkipChildren
		}

	case html.ErrorNode:
		return traverseFailed
	}

	return traverseOK
}

func parseIntOr(intv string, or int) int {
	v, _ := strconv.Atoi(intv)
	if v <= 0 {
		return or
	}
	return v
}

func (s *renderState) traverseChildren(n *html.Node) traverseStatus {
	return s.traverseSiblings(n.FirstChild)
}

func (s *renderState) traverseSiblings(first *html.Node) traverseStatus {
	for n := first; n != nil; n = n.NextSibling {
		switch s.renderNode(n) {
		case traverseOK:
			// traverseChildren never returns traverseSkipChildren.
			if s.traverseChildren(n) == traverseFailed {
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

// renderChildren renders the given node with the same tag name as its data using
// the given iterator. The iterator will be moved to the last written position
// when done.
func (s *renderState) renderChildren(n *html.Node) {
	s.renderChildrenTagName(n, n.Data)
}

// renderChildrenTagged is similar to renderChild, except the tag is given
// explicitly.
func (s *renderState) renderChildrenTagged(n *html.Node, tag *gtk.TextTag) {
	// There's a minor issue here: if, within the HTML, another block element
	// begins that creates another widget block, then the styling that we
	// obtained here will be lost. This is probably fine, since the HTML is
	// invalid if any of its styling carries across block elements, but it's
	// worth noting.
	text := s.block.text()
	start := text.iter.Offset()

	s.traverseSiblings(n.FirstChild)

	startIter := text.buf.IterAtOffset(start)
	text.buf.ApplyTag(tag, startIter, text.iter)
}

// renderChildrenTagName is similar to renderChildrenTagged, except the tag name
// is used.
func (s *renderState) renderChildrenTagName(n *html.Node, tagName string) {
	text := s.block.text()
	text.tagNameBounded(tagName, func() { s.traverseSiblings(n.FirstChild) })
}

// endLine ensures that either the current block is not a text block or there's
// a trailing new line in that text block. If the current block is not a text
// block, then a new text block is created.
func (s *renderState) endLine(n *html.Node, amount int) {
	// Ignore the line break if the next sibling is a block element, since those
	// will always be on a new line.
	if sibling := nodeNextSibling(n); sibling != nil && sibling.Type == html.ElementNode {
		switch sibling.Data {
		// This list is exhaustive enough; it's the only way we can guess if the
		// next element is a new block without actually progressing.
		case "p", "div", "pre", "blockquote":
			amount--
		}
	}

	if amount < 1 {
		return
	}

	switch block := s.block.current().(type) {
	case *textBlock:
		block.endLine(n, amount)
	case *quoteBlock:
		block.endLine(n, amount)
	case *codeBlock:
		block.text.endLine(n, amount)
	default:
		s.block.finalizeBlock()
	}
}

// nodeNextSibling returns the node's next sibling in the whole tree, not just
// in the current level. Nil is returned if the node is the last one in the
// tree.
func nodeNextSibling(n *html.Node) *html.Node {
	if n.NextSibling != nil {
		return n.NextSibling
	}

	for {
		parent := n.Parent
		if parent == nil {
			break
		}

		if parent.NextSibling != nil {
			// Parent still has something next to it.
			return parent.NextSibling
		}

		// Set the node as the parent. The above check will be repeated for it.
		n = parent
	}

	return nil
}

// nodePrevSibling returns the node's next sibling in the whole tree, not just
// in the current level. Nil is returned if the node is the last one in the
// tree.
func nodePrevSibling(n *html.Node) *html.Node {
	if n.PrevSibling != nil {
		return n.PrevSibling
	}

	for {
		parent := n.Parent
		if parent == nil {
			break
		}

		if parent.PrevSibling != nil {
			// Parent still has something next to it.
			return parent.PrevSibling
		}

		// Set the node as the parent. The above check will be repeated for it.
		n = parent
	}

	return nil
}

func nodeAttr(n *html.Node, keys ...string) string {
	for _, attr := range n.Attr {
		for _, k := range keys {
			if k == attr.Key {
				return attr.Val
			}
		}
	}
	return ""
}

func nodeHasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func nodePrependText(n *html.Node, text string) {
	node := &html.Node{
		Type: html.TextNode,
		Data: text,
	}
	n.InsertBefore(node, n.FirstChild)
}

func nodeText(n *html.Node) string {
	if n != nil && n.Type == html.TextNode {
		return n.Data
	}
	return ""
}

func nodeHasText(n *html.Node) bool {
	if strings.TrimSpace(nodeText(n)) != "" {
		return true
	}
	for n := n.FirstChild; n != nil; n = n.NextSibling {
		if nodeHasText(n) {
			return true
		}
	}
	return false
}
