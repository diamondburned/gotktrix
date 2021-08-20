// Package md provides Markdown helper functions as well as styling.
package md

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	markutil "github.com/yuin/goldmark/util"
)

// Parser is the default Markdown parser.
var Parser = parser.NewParser(
	parser.WithInlineParsers(
		markutil.Prioritized(parser.NewLinkParser(), 0),
		markutil.Prioritized(parser.NewEmphasisParser(), 1),
		markutil.Prioritized(parser.NewCodeSpanParser(), 2),
		markutil.Prioritized(parser.NewRawHTMLParser(), 3),
	),
	parser.WithBlockParsers(
		markutil.Prioritized(parser.NewParagraphParser(), 0),
		markutil.Prioritized(parser.NewBlockquoteParser(), 1),
		markutil.Prioritized(parser.NewATXHeadingParser(), 2),
		markutil.Prioritized(parser.NewFencedCodeBlockParser(), 3),
	),
)

// Converter is the default converter that outputs HTML.
var Converter = goldmark.New(goldmark.WithParser(Parser))

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
	"li": {
		"left-margin": 18, // px
	},

	// Not HTML tag.
	"_invisible": {"invisible": true},
	"_ligatures": {"font-features": "dlig=1"},
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
