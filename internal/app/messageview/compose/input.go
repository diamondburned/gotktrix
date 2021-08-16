package compose

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mcontent"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	markutil "github.com/yuin/goldmark/util"
)

// Input is the input component of the message composer.
type Input struct {
	*gtk.Box
	text *gtk.TextView
	send *gtk.Button

	buffer *gtk.TextBuffer

	ctx    context.Context
	roomID matrix.RoomID
}

var inputCSS = cssutil.Applier("composer-input", `
	.composer-input,
	.composer-input text {
		background-color: inherit;
	}
	.composer-input {
		padding: 12px 2px;
	}
`)

var sendCSS = cssutil.Applier("composer-send", `
	.composer-send {
		margin:   0px;
		padding: 10px;
		border-radius: 0;
	}
`)

func init() {
	mcontent.TextTags.Combine(markuputil.TextTagsMap{
		// Not HTML tags.
		"_htmltag": {
			"family":     "Monospace",
			"foreground": "#808080",
		},
	})
}

var defParser = parser.NewParser(
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

func parseAndWalk(src []byte, w ast.Walker) error {
	n := defParser.Parse(text.NewReader(src))
	return ast.Walk(n, w)
}

// NewInput creates a new Input instance.
func NewInput(ctx context.Context, roomID matrix.RoomID) *Input {
	tview := gtk.NewTextView()
	tview.SetWrapMode(gtk.WrapWordChar)
	tview.SetAcceptsTab(true)
	tview.SetHExpand(true)
	tview.SetInputHints(0 |
		gtk.InputHintEmoji |
		gtk.InputHintInhibitOSK |
		gtk.InputHintSpellcheck |
		gtk.InputHintWordCompletion |
		gtk.InputHintUppercaseSentences,
	)
	inputCSS(tview)

	buffer := tview.Buffer()
	buffer.Connect("changed", func() {
		head := buffer.StartIter()
		tail := buffer.EndIter()

		// Be careful to include anything hidden, since we want the offsets that
		// goldmark processes to be the exact same as what's in the buffer.
		input := []byte(buffer.Slice(&head, &tail, true))

		// Remove all tags before recreating them all.
		buffer.RemoveAllTags(&head, &tail)

		w := walker{buf: buffer}
		if err := parseAndWalk(input, w.walker); err != nil {
			log.Println("markdown input error:", err)
			return
		}
	})

	send := gtk.NewButtonFromIconName("document-send-symbolic")
	send.SetTooltipText("Send")
	send.SetHasFrame(false)
	send.SetSizeRequest(AvatarWidth, -1)
	sendCSS(send)

	enterKeyer := gtk.NewEventControllerKey()
	enterKeyer.Connect(
		"key-pressed",
		func(_ *gtk.EventControllerKey, val, code uint, state gdk.ModifierType) bool {
			switch val {
			case gdk.KEY_Return:
				// TODO: find a better way to do this. goldmark won't try to
				// parse an incomplete codeblock (I think), but the changed
				// signal will be fired after this signal.
				//
				// Perhaps we could use the FindChar method to avoid allocating
				// a new string (twice) on each keypress.
				head := buffer.StartIter()
				tail := buffer.IterAtOffset(buffer.ObjectProperty("cursor-position").(int))
				uinput := buffer.Text(&head, &tail, false)

				withinCodeblock := strings.Count(uinput, "```")%2 != 0

				// Enter (without holding Shift) sends the message.
				if !state.Has(gdk.ShiftMask) && !withinCodeblock {
					return send.Activate()
				}
			}

			return false
		},
	)

	tview.AddController(enterKeyer)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(tview)
	box.Append(send)

	return &Input{
		Box:    box,
		text:   tview,
		buffer: buffer,
		ctx:    ctx,
		roomID: roomID,
	}
}

type walker struct {
	buf   *gtk.TextBuffer
	table *gtk.TextTagTable

	head *gtk.TextIter
	tail *gtk.TextIter
}

func (w *walker) walker(n ast.Node, enter bool) (ast.WalkStatus, error) {
	if !enter {
		return ast.WalkContinue, nil
	}

	// Pre-allocate iters.
	if w.head == nil && w.tail == nil {
		head := w.buf.StartIter()
		tail := w.buf.EndIter()

		w.head = &head
		w.tail = &tail
	}

	return w.enter(n), nil
}

func (w *walker) enter(n ast.Node) ast.WalkStatus {
	switch n := n.(type) {
	case *ast.Emphasis:
		var tag string
		switch n.Level {
		case 1:
			tag = "i"
		case 2:
			tag = "b"
		default:
			return ast.WalkContinue
		}

		w.markText(n, tag)
		return ast.WalkSkipChildren

	case *ast.Heading:
		// h1 ~ h6
		if n.Level >= 1 && n.Level <= 6 {
			w.markTextFunc(n, []string{"h" + strconv.Itoa(n.Level)},
				func(head, tail *gtk.TextIter) {
					// Seek head to the start of the line to account for the
					// hash ("#").
					head.BackwardFindChar(func(ch uint32) bool { return rune(ch) == '\n' }, nil)
				},
			)
			return ast.WalkSkipChildren
		}

	case *ast.Link:
		w.markText(n, "a")
		return ast.WalkSkipChildren

	case *ast.CodeSpan:
		w.markText(n, "code")
		return ast.WalkSkipChildren

	case *ast.RawHTML:
		segments := n.Segments.Sliced(0, n.Segments.Len())
		for _, seg := range segments {
			w.markBounds(seg.Start, seg.Stop, "_htmltag")
		}

	case *ast.FencedCodeBlock:
		lines := n.Lines()
		if len := lines.Len(); len > 0 {
			w.markBounds(lines.At(0).Start, lines.At(len-1).Stop, "code")
			return ast.WalkSkipChildren
		}
	}

	return ast.WalkContinue
}

func (w *walker) tag(tagName string) *gtk.TextTag {
	if w.table == nil {
		w.table = w.buf.TagTable()
	}

	return mcontent.TextTags.FromTable(w.table, tagName)
}

func (w *walker) markBounds(i, j int, names ...string) {
	w.head.SetOffset(i)
	w.tail.SetOffset(j)

	for _, name := range names {
		w.buf.ApplyTag(w.tag(name), w.head, w.tail)
	}
}

// markText walks n's children and marks all its ast.Texts with the given tag.
func (w *walker) markText(n ast.Node, names ...string) {
	w.markTextFunc(n, names, nil)
}

// markTextFunc is similar to markText, except the caller has control over the
// head and tail iterators before the tags are applied. This is useful for block
// elements.
func (w *walker) markTextFunc(n ast.Node, names []string, f func(h, t *gtk.TextIter)) {
	walkChildren(n, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		text, ok := n.(*ast.Text)
		if !ok {
			return ast.WalkContinue, nil
		}

		w.head.SetOffset(text.Segment.Start)
		w.tail.SetOffset(text.Segment.Stop)

		if f != nil {
			f(w.head, w.tail)
		}

		for _, name := range names {
			w.buf.ApplyTag(w.tag(name), w.head, w.tail)
		}

		return ast.WalkContinue, nil
	})
}

// walkChildren walks n's children nodes using the given walker.
// WalkSkipChildren is returned unless the walker fails.
func walkChildren(n ast.Node, walker ast.Walker) ast.WalkStatus {
	for n := n.FirstChild(); n != nil; n = n.NextSibling() {
		ast.Walk(n, walker)
	}
	return ast.WalkSkipChildren
}
