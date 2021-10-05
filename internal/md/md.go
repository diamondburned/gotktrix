// Package md provides Markdown helper functions as well as styling.
package md

import (
	"context"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	markutil "github.com/yuin/goldmark/util"
)

// Parser is the default Markdown parser.
var Parser = parser.NewParser(
	parser.WithInlineParsers(
		markutil.Prioritized(parser.NewLinkParser(), 0),
		markutil.Prioritized(parser.NewAutoLinkParser(), 1),
		markutil.Prioritized(parser.NewEmphasisParser(), 2),
		markutil.Prioritized(parser.NewCodeSpanParser(), 3),
		markutil.Prioritized(parser.NewRawHTMLParser(), 4),
	),
	parser.WithBlockParsers(
		markutil.Prioritized(parser.NewParagraphParser(), 0),
		markutil.Prioritized(parser.NewBlockquoteParser(), 1),
		markutil.Prioritized(parser.NewATXHeadingParser(), 2),
		markutil.Prioritized(parser.NewFencedCodeBlockParser(), 3),
	),
)

var Renderer = html.NewRenderer(
	html.WithHardWraps(),
	html.WithUnsafe(),
)

// Converter is the default converter that outputs HTML.
var Converter = goldmark.New(
	goldmark.WithParser(Parser),
	goldmark.WithRenderer(
		renderer.NewRenderer(
			renderer.WithNodeRenderers(
				markutil.Prioritized(Renderer, 1000),
			),
		),
	),
)

// TabWidth is the width of a tab character in regular monospace characters.
var TabWidth = prefs.NewInt(4, prefs.PropMeta{
	Name:        "Tab Width",
	Description: "The tab width (in characters).",
})

var monospaceAttr = markuputil.Attrs(
	pango.NewAttrFamily("Monospace"),
)

// SetTabSize sets the given TextView's tab size.
func SetTabSize(text *gtk.TextView) {
	layout := text.CreatePangoLayout(" ")
	layout.SetAttributes(monospaceAttr)

	width, _ := layout.PixelSize()

	stops := pango.NewTabArray(1, true)
	stops.SetTab(0, pango.TabLeft, TabWidth.Value()*width)

	text.SetTabs(stops)
}

// TextTags contains the tag table mapping most Matrix HTML tags to GTK
// TextTags.
var TextTags = markuputil.TextTagsMap{
	// https://www.w3schools.com/cssref/css_default_values.asp
	"h1":     htag(1.75),
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
	"sup":    {"rise": +6000, "scale": 0.7},
	"sub":    {"rise": -2000, "scale": 0.7},
	"code": {
		"family":         "Monospace",
		"insert-hyphens": false,
	},
	"a": {
		"foreground":     "#238cf5",
		"insert-hyphens": false,
	},
	"a:hover": { // TODO
		"underline": pango.UnderlineSingle,
	},
	"a:visited": {
		"foreground": "#d38dff",
	},
	"caption": {
		"weight": pango.WeightLight,
		"style":  pango.StyleItalic,
		"scale":  0.8,
	},
	"li": {
		"left-margin": 24, // px
	},
	"blockquote": {
		"foreground":  "#789922",
		"left-margin": 12, // px
	},

	// Not HTML tag.
	"htmltag": {
		"family":     "Monospace",
		"foreground": "#808080",
	},
	// Meta tags.
	"_invisible": {"invisible": true},
	"_emoji":     {"scale": 2.5},
}

func htag(scale float64) markuputil.TextTag {
	return markuputil.TextTag{
		"scale":  scale,
		"weight": pango.WeightBold,
	}
}

var separatorCSS = cssutil.Applier("md-separator", `
	.md-separator {
		background-color: @theme_fg_color;
	}
`)

// NewSeparator creates a new 100px Markdown <hr> widget.
func NewSeparator() *gtk.Separator {
	s := gtk.NewSeparator(gtk.OrientationHorizontal)
	s.SetSizeRequest(100, -1)
	separatorCSS(s)
	return s
}

// AddWidgetAt adds a widget into the text view at the current iterator
// position.
func AddWidgetAt(text *gtk.TextView, iter *gtk.TextIter, w gtk.Widgetter) {
	anchor := text.Buffer().CreateChildAnchor(iter)
	text.AddChildAtAnchor(w, anchor)
}

// WalkChildren walks n's children nodes using the given walker.
// WalkSkipChildren is returned unless the walker fails.
func WalkChildren(n ast.Node, walker ast.Walker) ast.WalkStatus {
	for n := n.FirstChild(); n != nil; n = n.NextSibling() {
		ast.Walk(n, walker)
	}
	return ast.WalkSkipChildren
}

// ParseAndWalk parses src and walks its Markdown AST tree.
func ParseAndWalk(src []byte, w ast.Walker) error {
	n := Parser.Parse(text.NewReader(src))
	return ast.Walk(n, w)
}

// InsertInvisible inserts an invisible string of text into the buffer. This is
// useful for inserting invisible textual data during editing.
func InsertInvisible(pos *gtk.TextIter, txt string) {
	buf := pos.Buffer()
	insertInvisible(buf, pos, txt)
}

func insertInvisible(buf *gtk.TextBuffer, pos *gtk.TextIter, txt string) {
	tbl := buf.TagTable()
	tag := TextTags.FromTable(tbl, "_invisible")

	start := pos.Offset()
	buf.Insert(pos, txt, len(txt))

	startIter := buf.IterAtOffset(start)
	buf.ApplyTag(tag, &startIter, pos)
}

// AsyncInsertImage asynchronously inserts an image paintable. It does so in a
// way that the text position of the text buffer is not scrambled.
//
// Note that the caller should be careful when using this function: only modify
// the text buffer once the given context is cancelled. If that isn't done, then
// the function might incorrectly insert an image when it's not needed anymore.
// This is only a concern if the text buffer is mutable, however.
func AsyncInsertImage(ctx context.Context, iter *gtk.TextIter, url string, opts ...imgutil.Opts) {
	buf := iter.Buffer()

	offset := iter.Offset()
	// Insert a placeholder character right at the offset.
	insertInvisible(buf, iter, "\uFFFC")

	ctx, cancel := context.WithCancel(ctx)

	// Handle mutating the buffer.
	buf.Connect("changed", func(buf *gtk.TextBuffer) {
		iter := buf.IterAtOffset(offset)
		next := buf.IterAtOffset(offset + 1)

		if d := buf.Slice(&iter, &next, true); d != "\uFFFC" {
			cancel()
			return
		}
	})

	setImg := func(p gdk.Paintabler) {
		iter := buf.IterAtOffset(offset)
		next := buf.IterAtOffset(offset + 1)

		// Verify that the character at the buffer is still the intended one.
		if d := buf.Slice(&iter, &next, true); d != "\uFFFC" {
			// Character is different; don't modify the buffer.
			return
		}

		// Delete the 0xFFFC character that we temporarily inserted into
		// the buffer to reserve the offset.
		buf.Delete(&iter, &next)
		// Insert the pixbuf.
		buf.InsertPaintable(&iter, p)
		// Clean up the context.
		cancel()
	}

	imgutil.AsyncGET(ctx, url, setImg, opts...)
}

// https://stackoverflow.com/a/36258684/5041327
var emojiRanges = [][2]rune{
	{0x1F600, 0x1F64F}, // Emoticons
	{0x1F300, 0x1F5FF}, // Misc Symbols and Pictographs
	{0x1F680, 0x1F6FF}, // Transport and Map
	{0x2600, 0x26FF},   // Misc symbols
	{0x2700, 0x27BF},   // Dingbats
	{0xFE00, 0xFE0F},   // Variation Selectors
	{0x1F900, 0x1F9FF}, // Supplemental Symbols and Pictographs
	{0x1F1E6, 0x1F1FF}, // Flags
}

// IsUnicodeEmoji returns true if the given string only contains a Unicode
// emoji.
func IsUnicodeEmoji(v string) bool {
runeLoop:
	for {
		r, sz := utf8.DecodeRuneInString(v)
		if sz == 0 || r == utf8.RuneError {
			break
		}
		v = v[sz:]

		for _, crange := range emojiRanges {
			if crange[0] <= r && r <= crange[1] {
				continue runeLoop
			}
		}

		// runeLoop not hit; bail.
		return false
	}

	return true
}
