package text

import (
	"context"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/md"
	"github.com/diamondburned/gotktrix/internal/md/hl"
	"golang.org/x/net/html"
)

const (
	smallEmojiSize = 18
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
func RenderHTML(ctx context.Context, tview *gtk.TextView, text, html string) RenderMetadata {
	if md.IsUnicodeEmoji(html) {
		return RenderText(ctx, tview, html)
	}

	meta, ok := renderHTML(ctx, tview, html)
	if !ok {
		meta = RenderText(ctx, tview, text)
	}

	return meta
}

// renderHTML returns true if the HTML parsing and rendering is successful.
func renderHTML(ctx context.Context, tview *gtk.TextView, htmlBody string) (RenderMetadata, bool) {
	var meta RenderMetadata

	n, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		log.Println("invalid message HTML:", err)
		return meta, false
	}

	buf := tview.Buffer()
	iter := buf.StartIter()

	state := renderState{
		tview: tview,
		buf:   buf,
		table: buf.TagTable(),
		iter:  iter,
		ctx:   ctx,
		list:  0,
		// TODO: detect unicode emojis.
		large: !nodeHasText(n),
	}

	if state.traverseSiblings(n) == traverseFailed {
		return meta, false
	}

	if state.replyURL != "" {
		// The URL is guaranteed to have this suffix. The trimming will also
		// throw away the event prefix, so add it back.
		id := "!" + strings.TrimPrefix(state.replyURL, eventURLPrefix)
		// Scan everything up to the first slash.
		if end := strings.Index(id, "/"); end > -1 {
			id = id[:end]
		}
		meta.RefID = matrix.EventID(id)
	}

	meta.URLs = autolink(buf)
	return meta, true
}

type traverseStatus uint8

const (
	traverseOK traverseStatus = iota
	traverseSkipChildren
	traverseFailed
)

type renderState struct {
	tview *gtk.TextView
	buf   *gtk.TextBuffer
	table *gtk.TextTagTable
	iter  *gtk.TextIter

	ctx  context.Context
	list int

	replyURL string
	reply    bool

	quote bool // TODO: make into a counter
	large bool
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

// nTrailingNewLine counts the number of trailing new lines up to 2.
func (s *renderState) nTrailingNewLine() int {
	if !s.isNewLine() {
		return 0
	}

	seeker := s.iter.Copy()

	for i := 0; i < 2; i++ {
		if !seeker.BackwardChar() || rune(seeker.Char()) != '\n' {
			return i
		}
	}

	return 2
}

func (s *renderState) isNewLine() bool {
	if !s.iter.BackwardChar() {
		// empty buffer, so consider yes
		return true
	}

	// take the character, then undo the backward immediately
	char := rune(s.iter.Char())
	s.iter.ForwardChar()

	return char == '\n'
}

func (s *renderState) p(n *html.Node, f func()) {
	s.startLine(n, 1)
	f()
	s.endLine(n, 1)
}

func (s *renderState) startLine(n *html.Node, amount int) {
	amount -= s.nTrailingNewLine()
	if nodePrevSibling(n) != nil && amount > 0 {
		s.buf.Insert(s.iter, strings.Repeat("\n", amount))
	}
}

func (s *renderState) endLine(n *html.Node, amount int) {
	amount -= s.nTrailingNewLine()
	if nodeNextSibling(n) != nil && amount > 0 {
		s.buf.Insert(s.iter, strings.Repeat("\n", amount))
	}
}

type trimmedText struct {
	text  string
	left  int
	right int
}

func trimNewLines(str string) trimmedText {
	rhs := len(str) - len(strings.TrimRight(str, "\n"))
	str = strings.TrimRight(str, "\n")

	lhs := len(str) - len(strings.TrimLeft(str, "\n"))
	str = strings.TrimLeft(str, "\n")

	return trimmedText{str, lhs, rhs}
}

func (s *renderState) insertNewLines(n int) {
	if n < 1 {
		return
	}
	s.buf.Insert(s.iter, strings.Repeat("\n", n))
}

func (s *renderState) renderNode(n *html.Node) traverseStatus {
	switch n.Type {
	case html.TextNode:
		trimmed := trimNewLines(n.Data)

		// Make up the left-hand-side new lines.
		s.insertNewLines(trimmed.left - s.nTrailingNewLine())

		if trimmed.text == "" {
			// Ignore this segment entirely and don't write the right-trailing
			// new lines.
			return traverseOK
		}

		// If we're in a blockquote and we're on a new line, then write the meme
		// arrow.
		if s.quote && s.isNewLine() {
			s.buf.Insert(s.iter, "> ")
		}

		// Insert the trimmed string.
		s.buf.Insert(s.iter, trimmed.text)

		if nextNode := nodeNextSibling(n); nextNode != nil {
			// Only make up new lines if we still have nodes.
			s.insertNewLines(trimmed.right)
			// If this is not the last node and the next node is not a text
			// node, then we have to space out the elements.
			// if nextNode.Type != html.TextNode {
			// 	s.buf.Insert(s.iter, " ")
			// }
		}

		return traverseOK

	case html.ElementNode:
		switch n.Data {
		case "html", "body", "head":
			return traverseOK

		// Inline.
		case "font", "span": // data-mx-bg-color, data-mx-color
			s.renderChildrenTagged(
				n,
				markuputil.HashTag(s.buf.TagTable(), markuputil.TextTag{
					"foreground": nodeAttr(n, "data-mx-color", "color"),
					"background": nodeAttr(n, "data-mx-bg-color"),
				}),
			)
			return traverseSkipChildren

		// Inline.
		case "h1", "h2", "h3", "h4", "h5", "h6",
			"em", "i", "strong", "b", "u", "strike", "del", "sup", "sub", "caption":
			s.renderChildren(n)
			return traverseSkipChildren

		// Inline.
		case "code":
			start := s.iter.Offset()
			s.renderChildren(n)

			if lang := strings.TrimPrefix(nodeAttr(n, "class"), "language-"); lang != "" {
				startIter := s.buf.IterAtOffset(start)
				hl.Highlight(s.ctx, startIter, s.iter, lang)
			}

			return traverseSkipChildren

		// Block Elements.
		case "blockquote":
			s.quote = true
			s.renderChildren(n)
			s.quote = false
			s.endLine(n, 1)
			return traverseSkipChildren

		// Block Elements.
		case "p", "pre", "div":
			s.traverseChildren(n)
			s.endLine(n, 1)
			return traverseSkipChildren

		// Inline.
		case "a":
			start := -1

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

			// See if this is a user mention. If yes, then color it.
			isMention := strings.HasPrefix(href, mentionURLPrefix)

			if isMention {
				if n := n.FirstChild; n.Type == html.TextNode {
					// If this is a mention, then ensure that we have an at (@)
					// character.
					if !strings.HasPrefix(n.Data, "@") {
						n.Data = "@" + n.Data
					}
				}
			}

			if isMention || urlIsSafe(href) {
				// Only bother with adding a link tag if we know that the URL
				// has a safe scheme.
				start = s.iter.Offset()
			}

			s.renderChildren(n)

			if start > -1 {
				startIter := s.buf.IterAtOffset(start)
				end := s.iter.Offset()

				tag := s.emptyTag(embeddedURLPrefix + embedURL(start, end, href))
				s.buf.ApplyTag(tag, startIter, s.iter)

				if isMention {
					// Format the user ID; the trimming will trim the at symbol.
					uID := matrix.UserID("@" + strings.TrimPrefix(href, mentionURLPrefix))
					// Color the mention.
					col := mauthor.UserColor(uID, mauthor.WithWidgetColor(s.tview))
					tag := markuputil.HashTag(s.buf.TagTable(), markuputil.TextTag{
						"foreground": col,
						"background": col + "33", // alpha
					})
					s.buf.ApplyTag(tag, startIter, s.iter)
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

			s.buf.Insert(s.iter, "    "+bullet)
			s.renderChildren(n)
			return traverseSkipChildren

		case "hr":
			s.p(n, func() { md.AddWidgetAt(s.tview, s.iter, md.NewSeparator()) })
			return traverseOK
		case "br":
			s.endLine(n, 1)
			return traverseOK

		case "img": // width, height, alt, title, src(mxc)
			src := matrix.URL(nodeAttr(n, "src"))

			u, err := url.Parse(string(src))
			if err != nil || u.Scheme != "mxc" {
				// Ignore the image entirely.
				s.buf.InsertMarkup(s.iter, `<span fgalpha="50%">[image]</span>`)
				return traverseOK
			}

			// TODO: figure out a way to insert a nice text representation of an
			// emoji that's invisible, so it's clipboard-and-TTS friendly. This
			// way doesn't work.
			// s.insertInvisible(nodeAttr(n, "title"))

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
			md.AsyncInsertImage(s.ctx, s.iter, thumbnail, w, h)
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

// renderChildren renders the given node with the same tag name as its data using
// the given iterator. The iterator will be moved to the last written position
// when done.
func (s *renderState) renderChildren(n *html.Node) {
	s.renderChildrenTagName(n, n.Data)
}

// renderChildrenTagged is similar to renderChild, except the tag is given
// explicitly.
func (s *renderState) renderChildrenTagged(n *html.Node, tag *gtk.TextTag) {
	start := s.iter.Offset()
	s.traverseSiblings(n.FirstChild)

	startIter := s.buf.IterAtOffset(start)
	s.buf.ApplyTag(tag, startIter, s.iter)
}

func (s *renderState) emptyTag(tagName string) *gtk.TextTag {
	return emptyTag(s.table, tagName)
}

func emptyTag(table *gtk.TextTagTable, tagName string) *gtk.TextTag {
	if tag := table.Lookup(tagName); tag != nil {
		return tag
	}

	tag := gtk.NewTextTag(tagName)
	if !table.Add(tag) {
		log.Panicf("failed to add new tag %q", tagName)
	}

	return tag
}

func (s *renderState) tag(tagName string) *gtk.TextTag {
	return md.TextTags.FromTable(s.table, tagName)
}

// tagNameBounded wraps around tagBounded.
func (s *renderState) tagNameBounded(tagName string, f func()) {
	s.tagBounded(s.tag(tagName), f)
}

// tagBounded saves the current offset and calls f, expecting the function to
// use s.iter. Then, the tag with the given name is applied on top.
func (s *renderState) tagBounded(tag *gtk.TextTag, f func()) {
	start := s.iter.Offset()
	f()
	startIter := s.buf.IterAtOffset(start)
	s.buf.ApplyTag(tag, startIter, s.iter)
}

// renderChildrenTagName is similar to renderChildrenTagged, except the tag name
// is used.
func (s *renderState) renderChildrenTagName(n *html.Node, tagName string) {
	s.tagNameBounded(tagName, func() { s.traverseSiblings(n.FirstChild) })
}

// insertInvisible inserts the given invisible.
func (s *renderState) insertInvisible(str string) {
	s.tagNameBounded("_invisible", func() { s.buf.Insert(s.iter, str) })
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
