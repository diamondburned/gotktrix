package compose

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/md"
	"github.com/diamondburned/gotktrix/internal/gtkutil/md/hl"
	"github.com/pkg/errors"
	"github.com/yuin/goldmark/ast"
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
	md.TextTags.Combine(markuputil.TextTagsMap{
		// Not HTML tags.
		"_htmltag": {
			"family":     "Monospace",
			"foreground": "#808080",
		},
	})
}

func copyMessage(buffer *gtk.TextBuffer, roomID matrix.RoomID) event.RoomMessageEvent {
	head := buffer.StartIter()
	tail := buffer.EndIter()

	input := buffer.Text(&head, &tail, true)

	ev := event.RoomMessageEvent{
		RoomEventInfo: event.RoomEventInfo{RoomID: roomID},
		Body:          input,
		MsgType:       event.RoomMessageText,
	}

	var html strings.Builder

	if err := md.Converter.Convert([]byte(input), &html); err == nil {
		ev.Format = event.FormatHTML
		ev.FormattedBody = html.String()
	}

	return ev
}

func highlightBuffer(ctx context.Context, buffer *gtk.TextBuffer) {
	head := buffer.StartIter()
	tail := buffer.EndIter()

	// Be careful to include anything hidden, since we want the offsets that
	// goldmark processes to be the exact same as what's in the buffer.
	input := []byte(buffer.Slice(&head, &tail, true))

	// Remove all tags before recreating them all.
	buffer.RemoveAllTags(&head, &tail)

	w := walker{
		ctx: ctx,
		buf: buffer,
		src: input,
	}

	if err := md.ParseAndWalk(input, w.walker); err != nil {
		log.Println("markdown input error:", err)
		return
	}
}

// NewInput creates a new Input instance.
func NewInput(ctx context.Context, roomID matrix.RoomID) *Input {
	tview := gtk.NewTextView()
	tview.SetWrapMode(gtk.WrapWordChar)
	tview.SetAcceptsTab(true)
	tview.SetHExpand(true)
	tview.SetInputHints(0 |
		gtk.InputHintEmoji |
		gtk.InputHintSpellcheck |
		gtk.InputHintWordCompletion |
		gtk.InputHintUppercaseSentences,
	)
	inputCSS(tview)

	buffer := tview.Buffer()
	buffer.Connect("changed", func(buffer *gtk.TextBuffer) {
		highlightBuffer(ctx, buffer)
	})

	send := gtk.NewButtonFromIconName("document-send-symbolic")
	send.SetTooltipText("Send")
	send.SetHasFrame(false)
	send.SetSizeRequest(AvatarWidth, -1)
	sendCSS(send)

	send.Connect("activate", func() {
		ev := copyMessage(buffer, roomID)
		go func() {
			client := gotktrix.FromContext(ctx)
			_, err := client.RoomEventSend(ev.RoomID, ev.Type(), ev)
			if err != nil {
				app.Error(ctx, errors.Wrap(err, "failed to send message"))
			}
		}()
	})

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
	ctx   context.Context
	buf   *gtk.TextBuffer
	table *gtk.TextTagTable

	head *gtk.TextIter
	tail *gtk.TextIter

	src []byte
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

		len := lines.Len()
		if len == 0 {
			return ast.WalkSkipChildren
		}

		w.markBounds(lines.At(0).Start, lines.At(len-1).Stop, "code")

		if lang := string(n.Language(w.src)); lang != "" {
			// Use markBounds' head and tail iterators.
			hl.Highlight(w.ctx, w.head, w.tail, lang)
		}

		return ast.WalkSkipChildren
	}

	return ast.WalkContinue
}

func (w *walker) tag(tagName string) *gtk.TextTag {
	if w.table == nil {
		w.table = w.buf.TagTable()
	}

	return md.TextTags.FromTable(w.table, tagName)
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
	md.WalkChildren(n, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
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
