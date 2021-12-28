package text

import (
	"container/list"
	"context"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
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
	*gtk.Overlay
	context context.Context

	scroll *gtk.ScrolledWindow
	lang   *gtk.Label
	text   *textBlock
}

var codeBlockCSS = cssutil.Applier("mcontent-code-block", `
	.mcontent-code-block scrollbar {
		background: none;
		border:     none;
	}
	.mcontent-code-block:active scrollbar {
		opacity: 0.2;
	}
	.mcontent-code-block:not(.mcontent-code-block-expanded) scrollbar {
		opacity: 0;
	}
	.mcontent-code-block-text {
		font-family: monospace;
		padding: 4px 6px;
		padding-bottom: 0px; /* bottom-margin */
	}
	.mcontent-code-block-actions > *:not(label) {
		background-color: @theme_bg_color;
		margin-top:    4px;
		margin-right:  4px;
		margin-bottom: 4px;
	}
	.mcontent-code-block-language {
		font-family: monospace;
		font-size: 0.9em;
		margin: 0px 6px;
		color: mix(@theme_bg_color, @theme_fg_color, 0.85);
	}
	/*
	 * ease-in-out-gradient -steps 5 -min 0.2 -max 0 
	 * ease-in-out-gradient -steps 5 -min 0 -max 75 -f $'%.2fpx\n'
	 */
	.mcontent-code-block-voverflow .mcontent-code-block-cover {
		background-image: linear-gradient(
			to top,
			alpha(@theme_bg_color, 0.25) 0.00px,
			alpha(@theme_bg_color, 0.24) 2.40px,
			alpha(@theme_bg_color, 0.19) 19.20px,
			alpha(@theme_bg_color, 0.06) 55.80px,
			alpha(@theme_bg_color, 0.01) 72.60px
		);
	}
`)

var codeLowerHeight = prefs.NewInt(200, prefs.IntMeta{
	Name:    "Collapsed Codeblock Height",
	Section: "Text",
	Description: "The height of a collapsed codeblock." +
		" Long snippets of code will appear cropped.",
	Min: 50,
	Max: 5000,
})

var codeUpperHeight = prefs.NewInt(400, prefs.IntMeta{
	Name:    "Expanded Codeblock Height",
	Section: "Text",
	Description: "The height of an expanded codeblock." +
		" Codeblocks are either shorter than this or as tall." +
		" Ignored if this is lower than the collapsed height.",
	Min: 50,
	Max: 5000,
})

func init() { prefs.Order(codeLowerHeight, codeUpperHeight) }

func newCodeBlock(s *currentBlockState) *codeBlock {
	text := newTextBlock(s)
	text.AddCSSClass("mcontent-code-block-text")
	text.SetWrapMode(gtk.WrapNone)
	text.SetBottomMargin(18)

	sw := gtk.NewScrolledWindow()
	sw.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	sw.SetChild(text)

	language := gtk.NewLabel("")
	language.AddCSSClass("mcontent-code-block-language")
	language.SetHExpand(true)
	language.SetEllipsize(pango.EllipsizeEnd)
	language.SetSingleLineMode(true)
	language.SetXAlign(0)
	language.SetVAlign(gtk.AlignCenter)

	copy := gtk.NewButton()
	copy.SetIconName("edit-copy-symbolic")
	copy.SetTooltipText("Copy All")
	copy.ConnectClicked(func() {
		popover := gtk.NewPopover()
		popover.SetCanTarget(false)
		popover.SetAutohide(false)
		popover.SetChild(gtk.NewLabel("Copied!"))
		popover.SetPosition(gtk.PosLeft)
		popover.SetParent(copy)

		start, end := text.buf.Bounds()
		text := text.buf.Text(start, end, false)

		clipboard := gdk.DisplayGetDefault().Clipboard()
		clipboard.SetText(text)

		popover.Popup()
		glib.TimeoutSecondsAdd(3, func() {
			popover.Popdown()
			popover.Unparent()
		})
	})

	expand := gtk.NewToggleButton()
	expand.SetTooltipText("Toggle reveal code")

	actions := gtk.NewBox(gtk.OrientationHorizontal, 0)
	actions.AddCSSClass("mcontent-code-block-actions")
	actions.SetHAlign(gtk.AlignFill)
	actions.SetVAlign(gtk.AlignStart)
	actions.Append(language)
	actions.Append(copy)
	actions.Append(expand)

	clickOverlay := gtk.NewBox(gtk.OrientationVertical, 0)
	clickOverlay.Append(sw)
	// Clicking on the codeblock will click the button for us, but only if it's
	// collapsed.
	click := gtk.NewGestureClick()
	click.SetButton(gdk.BUTTON_PRIMARY)
	click.SetExclusive(true)
	click.Connect("pressed", func() bool {
		// TODO: don't handle this on a touchscreen.
		if !expand.Active() {
			expand.Activate()
			return true
		}
		return false
	})
	clickOverlay.AddController(click)

	overlay := gtk.NewOverlay()
	overlay.SetOverflow(gtk.OverflowHidden)
	overlay.SetChild(clickOverlay)
	overlay.AddOverlay(actions)
	overlay.SetMeasureOverlay(actions, true)
	overlay.AddCSSClass("frame")
	codeBlockCSS(overlay)

	// Lazily initialized in notify::upper below.
	var cover *gtk.Box
	coverSetVisible := func(visible bool) {
		if cover != nil {
			cover.SetVisible(visible)
		}
	}

	// Manually keep track of the expanded height, since SetMaxContentHeight
	// doesn't work (below issue).
	var maxHeight int
	var minHeight int

	vadj := text.VAdjustment()

	toggleExpand := func() {
		if expand.Active() {
			overlay.AddCSSClass("mcontent-code-block-expanded")
			expand.SetIconName("view-restore-symbolic")
			sw.SetCanTarget(true)
			sw.SetSizeRequest(-1, maxHeight)
			sw.SetMarginTop(actions.AllocatedHeight())
			language.SetOpacity(1)
			coverSetVisible(false)
		} else {
			overlay.RemoveCSSClass("mcontent-code-block-expanded")
			expand.SetIconName("view-fullscreen-symbolic")
			sw.SetCanTarget(false)
			sw.SetSizeRequest(-1, minHeight)
			sw.SetMarginTop(0)
			language.SetOpacity(0)
			coverSetVisible(true)
			// Restore scrolling when uncollapsed.
			vadj.SetValue(0)
		}
	}
	expand.ConnectClicked(toggleExpand)

	// Workaround for issue https://gitlab.gnome.org/GNOME/gtk/-/issues/3515.
	vadj.Connect("notify::upper", func() {
		upperHeight := codeUpperHeight.Value()
		lowerHeight := codeLowerHeight.Value()
		if upperHeight < lowerHeight {
			upperHeight = lowerHeight
		}

		upper := int(vadj.Upper())
		maxHeight = upper
		minHeight = upper

		if maxHeight > upperHeight {
			maxHeight = upperHeight
		}

		if minHeight > lowerHeight {
			minHeight = lowerHeight
			overlay.AddCSSClass("mcontent-code-block-voverflow")

			if cover == nil {
				// Use a fading gradient to let the user know (visually) that
				// there's still more code hidden. This isn't very accessible.
				cover = gtk.NewBox(gtk.OrientationHorizontal, 0)
				cover.AddCSSClass("mcontent-code-block-cover")
				cover.SetCanTarget(false)
				cover.SetVAlign(gtk.AlignFill)
				cover.SetHAlign(gtk.AlignFill)
				overlay.AddOverlay(cover)
			}
		}

		// Quite expensive when it's put here, but it's safer.
		toggleExpand()
	})

	return &codeBlock{
		Overlay: overlay,
		context: s.context,
		scroll:  sw,
		lang:    language,
		text:    text,
	}
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func (b *codeBlock) withHighlight(lang string, f func(*textBlock)) {
	b.lang.SetText(lang)

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

func (b *textBlock) block()      {}
func (b *codeBlock) block()      {}
func (b *quoteBlock) block()     {}
func (b *separatorBlock) block() {}
