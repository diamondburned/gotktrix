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
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
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

var textTags = markuputil.TextTagTableFactory(markuputil.TextTagsMap{
	// https://www.w3schools.com/cssref/css_default_values.asp
	"h1":     htag(2.00),
	"h2":     htag(1.50),
	"h3":     htag(1.17),
	"h4":     htag(1.00),
	"h5":     htag(0.83),
	"h6":     htag(0.67),
	"em":     {"style": pango.StyleItalic},
	"i":      {"style": pango.StyleItalic},
	"strong": {"weight": pango.WeightBold},
	"b":      {"weight": pango.WeightBold},
	"u":      {"underline": pango.UnderlineSingle},
	"strike": {"strikethrough": true},
	"del":    {"strikethrough": true},
	"sup":    {"rise": +1000, "scale": 0.50},
	"sub":    {"rise": -1000, "scale": 0.50},
	"code": {
		"family":         "Monospace",
		"insert-hyphens": false,
	},
	"a": {
		"foreground":     "#238cf5",
		"underline":      pango.UnderlineSingle,
		"insert-hyphens": false,
	},
	"caption": {
		"weight": pango.WeightLight,
		"style":  pango.StyleItalic,
		"scale":  0.8,
	},
	"invisible": {"invisible": true},
})

func htag(scale float64) markuputil.TextTag {
	return markuputil.TextTag{
		"scale":  scale,
		"weight": pango.WeightBold,
	}
}

func newTextContent(ctx context.Context, msg event.RoomMessageEvent) textContent {
	body := strings.Trim(msg.Body, "\n")

	buf := gtk.NewTextBuffer(textTags())

	text := gtk.NewTextViewWithBuffer(buf)
	text.SetCursorVisible(false)
	text.SetHExpand(true)
	text.SetEditable(false)
	text.SetWrapMode(gtk.WrapWordChar)
	textContentCSS(text)

	switch msg.Format {
	case event.FormatHTML:
		n, err := html.ParseFragment(strings.NewReader(msg.FormattedBody), nil)
		if err == nil && renderIntoBuffer(ctx, buf, n) {
			goto rendered
		}
	}

	// fallback
	buf.SetText(body, len(body))

rendered:
	return textContent{
		TextView: text,
	}
}

func (c textContent) content() {}

func renderIntoBuffer(ctx context.Context, buf *gtk.TextBuffer, nodes []*html.Node) bool {
	iter := buf.StartIter()

	state := renderState{
		buf:  buf,
		iter: &iter,
		ctx:  ctx,
		list: 0,
	}

	for _, n := range nodes {
		if state.traverseSiblings(n) == traverseFailed {
			return false
		}
	}

	// Trim trailing new lines.
	{
		tail := buf.EndIter()
		head := buf.StartIter()
		text := buf.Slice(&head, &iter, true)

		if strings.HasSuffix(text, "\n") {
			head.SetOffset(
				// Calculate the new tail to trim the rest off.
				tail.Offset() - (len(text) - len(strings.TrimRight(text, "\n"))),
			)
			buf.Delete(&head, &tail)
		}
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
	buf  *gtk.TextBuffer
	iter *gtk.TextIter

	ctx  context.Context
	list int
}

func (s *renderState) traverseSiblings(first *html.Node) traverseStatus {
	for n := first; n != nil; n = n.NextSibling {
		switch s.renderNode(n) {
		case traverseOK:
			// traverseSiblings never returns traverseSkipChildren.
			if s.traverseSiblings(n.FirstChild) == traverseFailed {
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

func (s *renderState) renderNode(n *html.Node) traverseStatus {
	switch n.Type {
	case html.TextNode:
		s.buf.Insert(s.iter, n.Data, len(n.Data))
		return traverseOK

	case html.ElementNode:
		switch n.Data {
		case "html", "body", "head":
			return traverseOK

		// Inline.
		case "font", "span": // data-mx-bg-color, data-mx-color
			s.renderChildrenTagged(
				n,
				hashTag(s.buf.TagTable(), markuputil.TextTag{
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
			// TODO: syntax highlighting
			s.renderChildren(n)
			return traverseSkipChildren

		// Block Elements.
		case "blockquote":
			// TODO: ">" prefix, green texting.
			fallthrough

		// Block Elements.
		case "p", "pre", "div":
			if n.PrevSibling != nil {
				// Prepend new line.
				s.buf.Insert(s.iter, "\n", 1)
			}

			s.traverseSiblings(n.FirstChild)

			if n.NextSibling != nil {
				// Append new line.
				s.buf.Insert(s.iter, "\n", 1)
			}

			return traverseSkipChildren

		// Inline.
		case "a": // name, target, href(http, https, ftp, mailto, magnet)
			s.buf.CreateMark(nodeAttr(n, "href"), s.iter, true)
			s.renderChildren(n)
			return traverseSkipChildren

		case "ul":
		case "ol": // start
		case "li":

		case "hr":
			s.buf.Insert(s.iter, "\n---\n", -1)
			return traverseOK
		case "br":
			s.buf.Insert(s.iter, "\n\n", -1)
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
			return s.traverseSiblings(n.FirstChild)
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
	start := s.iter.Offset()
	s.traverseSiblings(n.FirstChild)

	startIter := s.buf.IterAtOffset(start)
	s.buf.ApplyTagByName(n.Data, &startIter, s.iter)
}

// renderChildrenTagged is similar to renderChild, except the tag is given
// explicitly.
func (s *renderState) renderChildrenTagged(n *html.Node, tag *gtk.TextTag) {
	start := s.iter.Offset()
	s.traverseSiblings(n.FirstChild)

	startIter := s.buf.IterAtOffset(start)
	s.buf.ApplyTag(tag, &startIter, s.iter)
}

// insertInvisible inserts the given invisible.
func (s *renderState) insertInvisible(str string) {
	start := s.iter.Offset()
	s.buf.Insert(s.iter, str, len(str))

	startIter := s.buf.IterAtOffset(start)
	s.buf.ApplyTagByName("invisible", &startIter, s.iter)
}

func hashTag(table *gtk.TextTagTable, attrs markuputil.TextTag) *gtk.TextTag {
	hash := "custom:" + attrs.Hash()

	if t := table.Lookup(hash); t != nil {
		return t
	}

	tag := attrs.Tag(hash)

	if !table.Add(tag) {
		log.Panicf("text tag hash collision %q", hash)
	}

	return tag
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
