// Package md provides Markdown helper functions as well as styling.
package md

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"

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

// EmojiScale is the scale of Unicode emojis.
const EmojiScale = 2.5

// EmojiAttrs is the Pango attributes set for a label showing an emoji. It is
// kept the same as the _emoji tag in TextTags.
var EmojiAttrs = textutil.Attrs(
	pango.NewAttrScale(EmojiScale),
)

// TextTags contains the tag table mapping most Matrix HTML tags to GTK
// TextTags.
var TextTags = textutil.TextTagsMap{
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
	"_image":     {"rise": -2 * pango.SCALE},
	"_nohyphens": {"insert-hyphens": false},
}

func htag(scale float64) textutil.TextTag {
	return textutil.TextTag{
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

// inlineImageHeightOffset is kept in sync with the -0.35em subtraction above,
// because GTK behaves weirdly with how the height is done. It only matters for
// small inline images, though.
const inlineImageHeightOffset = -4

// InlineImage is an inline image.
type InlineImage struct {
	*gtk.Image
}

// SetSizeRequest sets the minimum size of the inline image.
func (i *InlineImage) SetSizeRequest(w, h int) {
	h += inlineImageHeightOffset
	i.Image.SetSizeRequest(w, h)
}

var inlineImageCSS = cssutil.Applier("md-inlineimage", `
	.md-inlineimage {
		margin-bottom: -0.45em;
	}
`)

// InsertImageWidget asynchronously inserts a new image widget. It does so in a
// way that the text position of the text buffer is not scrambled. Images
// created using this function will have the ".md-inlineimage" class.
func InsertImageWidget(view *gtk.TextView, anchor *gtk.TextChildAnchor) *InlineImage {
	image := gtk.NewImageFromIconName("image-x-generic-symbolic")
	inlineImageCSS(image)

	fixTextHeight(view, image)

	view.AddChildAtAnchor(image, anchor)
	view.AddCSSClass("md-hasimage")

	return &InlineImage{image}
}

func fixTextHeight(view *gtk.TextView, image *gtk.Image) {
	for _, class := range view.CSSClasses() {
		if class == "md-hasimage" {
			return
		}
	}

	gtkutil.OnFirstMap(image, func() {
		// Workaround to account for GTK's weird height allocating when a widget
		// is added. We're removing most of the excess empty padding with this.
		h := image.AllocatedHeight() * 95 / 100
		if h < 1 {
			return
		}

		cssutil.Applyf(view, `* { margin-bottom: -%dpx; }`, h)
	})
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
