package text

import (
	"container/list"
	"context"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/md"
	"github.com/diamondburned/gotktrix/internal/md/hl"
	"golang.org/x/net/html"
)

// currentBlock describes blocks of widgets that behave similarly to HTML block
// elements. It does not have any concept of nesting, however, and nested HTML
// blocks are flattened out, which will also erase its stylings.
type currentBlock interface {
	gtk.Widgetter
	block()
}

type currentBlockState struct {
	context context.Context
	parent  *gtk.Box
	list    *list.List
	element *list.Element
	table   *gtk.TextTagTable
}

func newBlockState(ctx context.Context, parent *gtk.Box) currentBlockState {
	return currentBlockState{
		context: ctx,
		parent:  parent,
		list:    list.New(),
		table:   gtk.NewTextTagTable(),
	}
}

func (s *currentBlockState) current() interface{} {
	if s.element != nil {
		return s.element.Value
	}
	return nil
}

// text returns the textBlock that is within any writable block.
func (s *currentBlockState) text() *textBlock {
	switch block := s.current().(type) {
	case *textBlock:
		return block
	case *codeBlock:
		return block.text
	case *quoteBlock:
		return &block.textBlock
	default:
		// Everything else is not text.
		return s.paragraph()
	}
}

// richText returns a stylable text block, which is either a regular text block
// or a quote block.
func (s *currentBlockState) richText() *textBlock {
	switch block := s.current().(type) {
	case *textBlock:
		return block
	case *quoteBlock:
		return &block.textBlock
	default:
		// Everything else is not text.
		return s.paragraph()
	}
}

// finalizeBlock finalizes the current block. Any later use of the state will
// create a new block.
func (s *currentBlockState) finalizeBlock() {
	s.element = nil
}

func (s *currentBlockState) paragraph() *textBlock {
	if block, ok := s.current().(*textBlock); ok {
		return block
	}

	block := newTextBlock(s)

	s.element = s.list.PushBack(block)
	s.parent.Append(block)

	return block
}

func (s *currentBlockState) code() *codeBlock {
	if block, ok := s.current().(*codeBlock); ok {
		return block
	}

	block := newCodeBlock(s)

	s.element = s.list.PushBack(block)
	s.parent.Append(block)

	return block
}

func (s *currentBlockState) quote() *quoteBlock {
	if block, ok := s.current().(*quoteBlock); ok {
		return block
	}

	block := newQuoteBlock(s)

	s.element = s.list.PushBack(block)
	s.parent.Append(block)

	return block
}

func (s *currentBlockState) separator() *separatorBlock {
	if block, ok := s.current().(*separatorBlock); ok {
		return block
	}

	block := newSeparatorBlock()

	s.element = s.list.PushBack(block)
	s.parent.Append(block)

	return block
}

// TODO: turn quoteBlock into a Box, and implement descend+ascend for it.
func (s *currentBlockState) descend() {}
func (s *currentBlockState) ascend()  {}

type textBlock struct {
	*gtk.TextView
	buf  *gtk.TextBuffer
	iter *gtk.TextIter

	table     *gtk.TextTagTable
	context   context.Context
	hyperlink bool
}

func newTextBlock(state *currentBlockState) *textBlock {
	text := textBlock{
		context: state.context,
		table:   state.table,
		buf:     gtk.NewTextBuffer(state.table),
	}
	text.iter = text.buf.StartIter()
	text.TextView = newTextView(state.context, text.buf)
	text.AddCSSClass("mcontent-text-block")
	return &text
}

var textContentCSS = cssutil.Applier("mcontent-text", `
	textview.mcontent-text,
	textview.mcontent-text text {
		background-color: transparent;
	}
`)

func newTextView(ctx context.Context, buf *gtk.TextBuffer) *gtk.TextView {
	tview := gtk.NewTextViewWithBuffer(buf)
	tview.AddCSSClass("mcontent-text")
	tview.SetEditable(false)
	tview.SetCursorVisible(false)
	tview.SetHExpand(true)
	tview.SetVExpand(true)
	tview.SetWrapMode(gtk.WrapWordChar)

	textContentCSS(tview)
	md.SetTabSize(tview)

	return tview
}

// hasLink connects the needed handlers into the textBlock to handle hyperlinks.
func (b *textBlock) hasLink() {
	if b.hyperlink {
		return
	}

	b.hyperlink = true

	BindLinkHandler(b.TextView, func(url string) {
		app.OpenURI(b.context, url)
	})
}

// nTrailingNewLine counts the number of trailing new lines up to 2.
func (b *textBlock) nTrailingNewLine() int {
	if !b.isNewLine() {
		return 0
	}

	seeker := b.iter.Copy()

	for i := 0; i < 2; i++ {
		if !seeker.BackwardChar() || rune(seeker.Char()) != '\n' {
			return i
		}
	}

	return 2
}

func (b *textBlock) isNewLine() bool {
	if !b.iter.BackwardChar() {
		// empty buffer, so consider yes
		return true
	}

	// take the character, then undo the backward immediately
	char := rune(b.iter.Char())
	b.iter.ForwardChar()

	return char == '\n'
}

func (b *textBlock) p(n *html.Node, f func()) {
	b.startLine(n, 1)
	f()
	b.endLine(n, 1)
}

func (b *textBlock) startLine(n *html.Node, amount int) {
	amount -= b.nTrailingNewLine()
	if nodePrevSibling(n) != nil && amount > 0 {
		b.buf.Insert(b.iter, strings.Repeat("\n", amount))
	}
}

func (b *textBlock) endLine(n *html.Node, amount int) {
	amount -= b.nTrailingNewLine()
	if nodeNextSibling(n) != nil && amount > 0 {
		b.buf.Insert(b.iter, strings.Repeat("\n", amount))
	}
}

func (b *textBlock) emptyTag(tagName string) *gtk.TextTag {
	return emptyTag(b.table, tagName)
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

func (b *textBlock) tag(tagName string) *gtk.TextTag {
	return md.TextTags.FromTable(b.table, tagName)
}

// tagNameBounded wraps around tagBounded.
func (b *textBlock) tagNameBounded(tagName string, f func()) {
	b.tagBounded(b.tag(tagName), f)
}

// tagBounded saves the current offset and calls f, expecting the function to
// use s.iter. Then, the tag with the given name is applied on top.
func (b *textBlock) tagBounded(tag *gtk.TextTag, f func()) {
	start := b.iter.Offset()
	f()
	startIter := b.buf.IterAtOffset(start)
	b.buf.ApplyTag(tag, startIter, b.iter)
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

func (b *textBlock) insertNewLines(n int) {
	if n < 1 {
		return
	}
	b.buf.Insert(b.iter, strings.Repeat("\n", n))
}

// insertInvisible inserts the given invisible.
func (b *textBlock) insertInvisible(str string) {
	b.tagNameBounded("_invisible", func() { b.buf.Insert(b.iter, str) })
}

type quoteBlock struct {
	textBlock
}

var quoteBlockCSS = cssutil.Applier("mcontent-quote-block", `
	.mcontent-quote-block {
		border-left:  3px solid alpha(@theme_fg_color, 0.5);
		padding-left: 6px;
	}
	.mcontent-quote-block:not(:first-child) {
		margin-top: 3px;
	}
	.mcontent-quote-block:not(:last-child) {
		margin-bottom: 3px;
	}
`)

func newQuoteBlock(s *currentBlockState) *quoteBlock {
	quote := &quoteBlock{textBlock: *newTextBlock(s)}
	quote.AddCSSClass("mcontent-quote-block")
	return quote
}

type codeBlock struct {
	*gtk.ScrolledWindow
	context context.Context
	text    *textBlock
}

var codeBlockCSS = cssutil.Applier("mcontent-code-block", `
	.mcontent-code-block {
		background-color: @theme_base_color;
	}
	.mcontent-code-block scrollbar {
		background: none;
		border:     none;
	}
	.mcontent-code-block:active scrollbar {
		opacity: 0.2;
	}
	.mcontent-code-block-text {
		font-family: monospace;
		padding: 4px;
		padding-bottom: 8px;
	}
`)

func newCodeBlock(s *currentBlockState) *codeBlock {
	text := newTextBlock(s)
	text.AddCSSClass("mcontent-code-block-text")
	text.SetWrapMode(gtk.WrapNone)

	// TODO: this has the annoying quirk of stealing the scroll on a large
	// codeblock. It's pretty bad.
	sw := gtk.NewScrolledWindow()
	sw.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyNever)
	sw.SetChild(text)
	codeBlockCSS(sw)

	return &codeBlock{
		ScrolledWindow: sw,
		context:        s.context,
		text:           text,
	}
}

func (b *codeBlock) withHighlight(lang string, f func(*textBlock)) {
	if lang == "" {
		f(b.text)
		return
	}

	start := b.text.iter.Offset()
	f(b.text)

	startIter := b.text.buf.IterAtOffset(start)
	hl.Highlight(b.context, startIter, b.text.iter, lang)
}

type separatorBlock struct {
	*gtk.Separator
}

func newSeparatorBlock() *separatorBlock {
	sep := gtk.NewSeparator(gtk.OrientationHorizontal)
	sep.AddCSSClass("mcontent-separator-block")
	return &separatorBlock{sep}
}

func (b *textBlock) block()  {}
func (b *codeBlock) block()  {}
func (b *quoteBlock) block() {}
