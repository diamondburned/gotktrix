package md

import (
	"context"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/md/hl"
	"github.com/yuin/goldmark/ast"
)

// WYSIWYG styles the given text buffer according to the Markdown content inside
// it. It is not fully What-You-See-Is-What-You-Get, but it is mostly so.
func WYSIWYG(ctx context.Context, buffer *gtk.TextBuffer) {
	head := buffer.StartIter()
	tail := buffer.EndIter()

	// Be careful to include anything hidden, since we want the offsets that
	// goldmark processes to be the exact same as what's in the buffer.
	input := []byte(buffer.Slice(&head, &tail, true))

	w := wysiwyg{
		ctx:   ctx,
		buf:   buffer,
		table: buffer.TagTable(),
		src:   input,
		head:  &head,
		tail:  &tail,
	}

	removeTags := make([]*gtk.TextTag, 0, w.table.Size())

	w.table.Foreach(func(tag *gtk.TextTag) {
		// DO NOT REMOVE INVISIBLE TAGS! They're used by the caller for
		// additional data and should NEVER used by us.
		if tag.ObjectProperty("name").(string) != "_invisible" {
			removeTags = append(removeTags, tag)
		}
	})

	// Ensure that the WYSIWYG tags are all gone.
	for _, tag := range removeTags {
		buffer.RemoveTag(tag, &head, &tail)
	}

	// Error is not important.
	ParseAndWalk(input, w.walker)
}

// wysiwyg is the What-You-See-Is-What-You-Get node walker/highlighter.
type wysiwyg struct {
	ctx   context.Context
	buf   *gtk.TextBuffer
	table *gtk.TextTagTable

	head *gtk.TextIter
	tail *gtk.TextIter

	src []byte
}

func (w *wysiwyg) walker(n ast.Node, enter bool) (ast.WalkStatus, error) {
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

func (w *wysiwyg) enter(n ast.Node) ast.WalkStatus {
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

func (w *wysiwyg) tag(tagName string) *gtk.TextTag {
	if w.table == nil {
		w.table = w.buf.TagTable()
	}

	return TextTags.FromTable(w.table, tagName)
}

func (w *wysiwyg) markBounds(i, j int, names ...string) {
	w.head.SetOffset(i)
	w.tail.SetOffset(j)

	for _, name := range names {
		w.buf.ApplyTag(w.tag(name), w.head, w.tail)
	}
}

// markText walks n's children and marks all its ast.Texts with the given tag.
func (w *wysiwyg) markText(n ast.Node, names ...string) {
	w.markTextFunc(n, names, nil)
}

// markTextFunc is similar to markText, except the caller has control over the
// head and tail iterators before the tags are applied. This is useful for block
// elements.
func (w *wysiwyg) markTextFunc(n ast.Node, names []string, f func(h, t *gtk.TextIter)) {
	WalkChildren(n, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
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
