package mcontent

import (
	"context"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/md"
	"github.com/diamondburned/gotktrix/internal/gtkutil/md/hl"
	"github.com/diamondburned/gotktrix/internal/gtkutil/mediautil"
	"golang.org/x/net/html"
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

func newTextContent(ctx context.Context, msg event.RoomMessageEvent) textContent {
	body := strings.Trim(msg.Body, "\n")

	text := gtk.NewTextView()
	text.SetHExpand(true)
	text.SetEditable(false)
	text.SetAcceptsTab(false)
	text.SetCursorVisible(false)
	text.SetWrapMode(gtk.WrapWordChar)
	text.Show()
	textContentCSS(text)

	switch msg.Format {
	case event.FormatHTML:
		n, err := html.Parse(strings.NewReader(msg.FormattedBody))
		if err == nil && renderIntoBuffer(ctx, text, n) {
			goto rendered
		}
	}

	// fallback
	text.Buffer().SetText(body, len(body))

rendered:
	return textContent{
		TextView: text,
	}
}

func (c textContent) content() {}

func renderIntoBuffer(ctx context.Context, tview *gtk.TextView, node *html.Node) bool {
	buf := tview.Buffer()
	iter := buf.StartIter()

	state := renderState{
		tview: tview,
		buf:   buf,
		table: buf.TagTable(),
		iter:  &iter,
		ctx:   ctx,
		list:  0,
	}

	if state.traverseSiblings(node) == traverseFailed {
		return false
	}

	// Trim trailing new lines.
	trimBufferNewLineRight(buf)
	trimBufferNewLineLeft(buf)

	return true
}

func trimBufferNewLineRight(buf *gtk.TextBuffer) {
	tail := buf.EndIter()
	// Move from the end to the last character.
	if !tail.BackwardChar() {
		return
	}
	if rune(tail.Char()) != '\n' {
		return
	}

	head := buf.IterAtOffset(tail.Offset() - 1)
	buf.Delete(&head, &tail)

	// 	text := buf.Slice(&head, &tail, true)
	// 	if !strings.HasSuffix(text, "\n") {
	// 		return &tail
	// 	}

	// 	trim := len(strings.TrimRight(text, "\n"))

	// 	log.Printf("text      = %q", text)
	// 	log.Printf("trimming  = %d", trim)
	// 	log.Printf("trim text = %q", text[:trim])
	// 	log.Printf("end bound = %d", tail.Offset()-(len(text)-len(strings.TrimRight(text, "\n"))))

	// 	// Calculate the new tail to trim the rest off.
	// 	head.SetOffset(trim)
	// 	buf.Delete(&head, &tail)

	// 	return &tail
}

func trimBufferNewLineLeft(buf *gtk.TextBuffer) {
	head := buf.StartIter()
	if rune(head.Char()) != '\n' {
		return
	}

	tail := buf.IterAtOffset(1)
	buf.Delete(&head, &tail)

	// tail := buf.EndIter()
	// head := buf.StartIter()
	// text := buf.Slice(&head, &tail, true)

	// if !strings.HasPrefix(text, "\n") {
	// 	return &head
	// }

	// // Calculate the new tail to trim the rest off.
	// tail.SetOffset(len(text) - len(strings.TrimLeft(text, "\n")))
	// buf.Delete(&head, &tail)

	// return &head
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
	head := s.buf.IterAtOffset(s.iter.Offset() - 2)
	text := s.buf.Slice(&head, s.iter, true)
	return strings.Count(text, "\n")
}

// p starts a paragraph by padding the current text with at most 2 new lines.
func (s *renderState) p() {
	n := s.nTrailingNewLine()
	if n < 2 {
		s.buf.Insert(s.iter, strings.Repeat("\n", 2-n), -1)
	}
}

// line ensures that we're on a new line.
func (s *renderState) line() {
	if s.nTrailingNewLine() == 0 {
		s.buf.Insert(s.iter, "\n", 1)
	}
}

func trimNewLines(str string) (string, int) {
	new := strings.TrimRight(str, "\n")
	return new, len(str) - len(new)
}

func (s *renderState) renderNode(n *html.Node) traverseStatus {
	switch n.Type {
	case html.TextNode:
		text, newLines := trimNewLines(n.Data)
		s.buf.Insert(s.iter, text, len(text))

		// Calculate the actual number of new lines that we need while
		// accounting for ones that are already in the buffer.
		if n := s.nTrailingNewLine(); n > 0 {
			newLines -= n
		}
		if newLines > 0 {
			s.buf.Insert(s.iter, strings.Repeat("\n", newLines), -1)
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
			// TODO: ">" prefix, green texting.
			fallthrough

		// Block Elements.
		case "p", "pre", "div":
			s.traverseChildren(n)
			s.p()
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
			s.line()
			md.AddWidgetAt(s.tview, s.iter, md.NewSeparator())
			s.line()
			return traverseOK
		case "br":
			s.p()
			return traverseOK

		case "img": // width, height, alt, title, src(mxc)
			src := matrix.URL(nodeAttr(n, "src"))

			u, err := url.Parse(string(src))
			if err != nil || u.Scheme != "mxc" {
				// Ignore the image entirely.
				s.buf.InsertMarkup(s.iter, `<span fgalpha="50%">[image]</span>`, -1)
				return traverseOK
			}

			offset := s.iter.Offset()
			s.insertInvisible(nodeAttr(n, "title"))

			reqw := parseIntOr(nodeAttr(n, "width"), maxWidth)
			reqh := parseIntOr(nodeAttr(n, "height"), maxHeight)
			w, h := mediautil.MaxSize(reqw, reqh, maxWidth, maxHeight)

			thumbnail, _ := gotktrix.FromContext(s.ctx).Offline().Thumbnail(src, w, h)
			imgutil.AsyncGET(s.ctx, thumbnail, func(p gdk.Paintabler) {
				iter := s.buf.IterAtOffset(offset)
				s.buf.InsertPaintable(&iter, p)
			})
			return traverseOK

		default:
			log.Println("unknown tag", n.Data)
			return s.traverseChildren(n)
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

// renderChildrenTagName is similar to renderChildrenTagged, except the tag name
// is used.
func (s *renderState) renderChildrenTagName(n *html.Node, tagName string) {
	start := s.iter.Offset()
	s.traverseSiblings(n.FirstChild)

	startIter := s.buf.IterAtOffset(start)
	s.buf.ApplyTag(s.tag(tagName), &startIter, s.iter)
}

// insertInvisible inserts the given invisible.
func (s *renderState) insertInvisible(str string) {
	start := s.iter.Offset()
	s.buf.Insert(s.iter, str, len(str))

	startIter := s.buf.IterAtOffset(start)
	s.buf.ApplyTag(s.tag("_invisible"), &startIter, s.iter)
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

func nodePrependText(n *html.Node, text string) {
	node := &html.Node{
		Type: html.TextNode,
		Data: text,
	}
	n.InsertBefore(node, n.FirstChild)
}
