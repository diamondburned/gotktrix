// Package md provides Markdown helper functions as well as styling.
package md

import (
	"context"

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
		markutil.Prioritized(parser.NewThematicBreakParser(), 4), // <hr>
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
var TabWidth = prefs.NewInt(4, prefs.IntMeta{
	PropMeta: prefs.PropMeta{
		Name:        "Tab Width",
		Section:     "Appearance",
		Description: "The tab width (in characters).",
	},
	Min: 0,
	Max: 16,
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

// EmojiScale is the scale of Unicode emojis.
const EmojiScale = 2.5

// EmojiAttrs is the Pango attributes set for a label showing an emoji. It is
// kept the same as the _emoji tag in TextTags.
var EmojiAttrs = markuputil.Attrs(
	pango.NewAttrScale(EmojiScale),
)

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
	"_invisible": {"editable": false, "invisible": true},
	"_immutable": {"editable": false},
	"_emoji":     {"scale": EmojiScale},
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

// BeginImmutable begins the immutability region in the text buffer that the
// text iterator belongs to. Calling the returned callback will end the
// immutable region. Calling it is not required, but the given iterator must
// still be valid when it's called.
func BeginImmutable(pos *gtk.TextIter) func() {
	ix := pos.Offset()

	return func() {
		buf := pos.Buffer()
		tbl := buf.TagTable()
		tag := TextTags.FromTable(tbl, "_immutable")
		buf.ApplyTag(tag, buf.IterAtOffset(ix), pos)
	}
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
	buf.Insert(pos, txt)

	startIter := buf.IterAtOffset(start)
	buf.ApplyTag(tag, startIter, pos)
}

// AsyncInsertImage asynchronously inserts an image paintable. It does so in a
// way that the text position of the text buffer is not scrambled.
//
// Note that the caller should be careful when using this function: only modify
// the text buffer once the given context is cancelled. If that isn't done, then
// the function might incorrectly insert an image when it's not needed anymore.
// This is only a concern if the text buffer is mutable, however.
func AsyncInsertImage(
	ctx context.Context, iter *gtk.TextIter, url string, w, h int, opts ...imgutil.Opts) {

	buf := iter.Buffer()
	mark := buf.CreateMark("", iter, false)

	setImg := func(p gdk.Paintabler) {
		if p != nil && !mark.Deleted() {
			// Insert the pixbuf at the location if mark is not deleted.
			buf.InsertPaintable(buf.IterAtMark(mark), p)
		}
	}

	if w > 0 && h > 0 {
		opts = append(opts,
			imgutil.WithRescale(w, h),
			imgutil.WithFallbackIcon("dialog-error"),
		)
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

const minEmojiUnicode = 0x2600 // see above

// IsUnicodeEmoji returns true if the given string only contains a Unicode
// emoji.
func IsUnicodeEmoji(v string) bool {
runeLoop:
	for _, r := range v {
		// Fast path: only run the loop if this character is in any of the
		// ranges by checking the minimum rune.
		if r >= minEmojiUnicode {
			for _, crange := range emojiRanges {
				if crange[0] <= r && r <= crange[1] {
					continue runeLoop
				}
			}
		}
		// runeLoop not hit; bail.
		return false
	}
	return true
}
