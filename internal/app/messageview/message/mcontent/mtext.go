package mcontent

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/md"
	"github.com/diamondburned/gotktrix/internal/md/hl"
	"golang.org/x/net/html"
)

const (
	smallEmojiSize = 18
	largeEmojiSize = 48
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

const editedHTML = `<span alpha="75%" size="small">(edited)</span>`

func newTextContent(ctx context.Context, msgBox *gotktrix.EventBox) textContent {
	text := gtk.NewTextView()
	text.SetHExpand(true)
	text.SetEditable(false)
	text.SetAcceptsTab(false)
	text.SetCursorVisible(false)
	text.SetWrapMode(gtk.WrapWordChar)

	md.SetTabSize(text)
	textContentCSS(text)

	body, isEdited := msgBody(msgBox)

	switch body.Format {
	case event.FormatHTML:
		// Hit the fallback case if the HTML is just a Unicode emoji.
		if md.IsUnicodeEmoji(body.FormattedBody) {
			renderText(ctx, text, body.FormattedBody)
			break
		}
		if !renderHTML(ctx, text, body.FormattedBody) {
			// HTML failed; use text instead.
			renderText(ctx, text, body.Body)
		}
	default:
		renderText(ctx, text, body.Body)
	}

	if isEdited {
		buf := text.Buffer()
		end := buf.EndIter()

		append := editedHTML
		if buf.CharCount() > 0 {
			// Prepend a space if we already have text.
			append = " " + editedHTML
		}

		buf.InsertMarkup(&end, append, len(append))
	}

	text.Connect("map", func() {
		// Fixes 2 GTK bugs:
		// - TextViews are invisible sometimes.
		// - Multiline TextViews are sometimes only drawn as 1.
		glib.IdleAdd(func() {
			text.QueueAllocate()
			text.QueueResize()
		})
	})

	return textContent{
		TextView: text,
	}
}

func (c textContent) content() {}

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

// renderText renders the given plain text.
func renderText(ctx context.Context, tview *gtk.TextView, text string) {
	body := strings.Trim(text, "\n")
	tbuf := tview.Buffer()
	tbuf.SetText(body, len(body))

	if md.IsUnicodeEmoji(body) {
		start, end := tbuf.Bounds()
		tbuf.ApplyTag(md.TextTags.FromTable(tbuf.TagTable(), "_emoji"), &start, &end)
	}
}

// renderHTML returns true if the HTML parsing and rendering is successful.
func renderHTML(ctx context.Context, tview *gtk.TextView, htmlBody string) bool {
	n, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		return false
	}

	buf := tview.Buffer()
	iter := buf.StartIter()

	state := renderState{
		tview: tview,
		buf:   buf,
		table: buf.TagTable(),
		iter:  &iter,
		ctx:   ctx,
		list:  0,
		// TODO: detect unicode emojis.
		large: !nodeHasText(n),
	}

	if state.traverseSiblings(n) == traverseFailed {
		return false
	}

	return true
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

	ctx   context.Context
	list  int
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
	seeker := s.iter.Copy()

	for i := 0; i < 2; i++ {
		if !seeker.BackwardChar() || rune(seeker.Char()) != '\n' {
			return i
		}
	}

	return 2
}

func (s *renderState) p(n *html.Node, f func()) {
	s.startLine(n, 1)
	f()
	s.endLine(n, 1)
}

func (s *renderState) startLine(n *html.Node, amount int) {
	amount -= s.nTrailingNewLine()
	if nodePrevSibling(n) != nil && amount > 0 {
		s.buf.Insert(s.iter, strings.Repeat("\n", amount), amount)
	}
}

func (s *renderState) endLine(n *html.Node, amount int) {
	amount -= s.nTrailingNewLine()
	if nodeNextSibling(n) != nil && amount > 0 {
		s.buf.Insert(s.iter, strings.Repeat("\n", amount), amount)
	}
}

func trimNewLines(str string) (string, int) {
	new := strings.TrimRightFunc(str, unicode.IsSpace)
	lns := len(str) - len(strings.TrimRight(str, "\n"))
	// Cap new lines at 2.
	if lns > 2 {
		lns = 2
	}
	return new, lns
}

func (s *renderState) renderNode(n *html.Node) traverseStatus {
	switch n.Type {
	case html.TextNode:
		text, newLines := trimNewLines(n.Data)
		s.buf.Insert(s.iter, text, len(text))

		nextNode := nodeNextSibling(n)

		// Calculate the actual number of new lines that we need while
		// accounting for ones that are already in the buffer.
		if n := s.nTrailingNewLine(); n > 0 {
			newLines -= n
		}
		// Only make up new lines if we still have nodes.
		if newLines > 0 && nextNode != nil {
			s.buf.Insert(s.iter, strings.Repeat("\n", newLines), newLines)
		}

		// If this is not the last node and the next node is not a text node,
		// then we have to space out the elements.
		if nextNode != nil && nextNode.Type != html.TextNode {
			s.buf.Insert(s.iter, " ", 1)
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
				hl.Highlight(s.ctx, &startIter, s.iter, lang)
			}

			return traverseSkipChildren

		// Block Elements.
		case "blockquote":
			s.renderChildren(n)
			s.endLine(n, 1)
			return traverseSkipChildren

		// Block Elements.
		case "p", "pre", "div":
			s.traverseChildren(n)
			s.endLine(n, 1)
			return traverseSkipChildren

		// Inline.
		case "a": // name, target, href(http, https, ftp, mailto, magnet)
			s.buf.CreateMark(nodeAttr(n, "href"), s.iter, true)
			s.renderChildren(n)
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

			nodePrependText(n, bullet)
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
				s.buf.InsertMarkup(s.iter, `<span fgalpha="50%">[image]</span>`, -1)
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

			thumbnail, _ := gotktrix.FromContext(s.ctx).Offline().ScaledThumbnail(src, w, h)
			md.AsyncInsertImage(s.ctx, s.iter, thumbnail, imgutil.WithRescale(w, h))
			return traverseOK

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
	s.buf.ApplyTag(tag, &startIter, s.iter)
}

func (s *renderState) tag(tagName string) *gtk.TextTag {
	return md.TextTags.FromTable(s.table, tagName)
}

// tagBounded saves the current offset and calls f, expecting the function to
// use s.iter. Then, the tag with the given name is applied on top.
func (s *renderState) tagBounded(tagName string, f func()) {
	start := s.iter.Offset()
	f()
	startIter := s.buf.IterAtOffset(start)
	s.buf.ApplyTag(s.tag(tagName), &startIter, s.iter)
}

// renderChildrenTagName is similar to renderChildrenTagged, except the tag name
// is used.
func (s *renderState) renderChildrenTagName(n *html.Node, tagName string) {
	s.tagBounded(tagName, func() { s.traverseSiblings(n.FirstChild) })
}

// insertInvisible inserts the given invisible.
func (s *renderState) insertInvisible(str string) {
	s.tagBounded("_invisible", func() { s.buf.Insert(s.iter, str, len(str)) })
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

func nodeHasText(n *html.Node) bool {
	if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
		return true
	}
	for n := n.FirstChild; n != nil; n = n.NextSibling {
		if nodeHasText(n) {
			return true
		}
	}
	return false
}
