package logui

import (
	"context"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/components/autoscroll"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

var (
	defaultOnce   sync.Once
	defaultBuffer *Buffer
)

// BindLogger binds the given logger to the buffer.
func BindLogger(logger *log.Logger, buffer *Buffer) {
	w := io.MultiWriter(logger.Writer(), buffer)
	logger.SetOutput(w)
}

// BindDefaultLogger binds the default logger to the default buffer.
func BindDefaultLogger() {
	BindLogger(log.Default(), DefaultBuffer())
}

// MaxChars is the default maximum amount of characters of any log buffer. It's
// set to 1 million by default. When decoded to full characters, that's 4MB.
const MaxChars = 1_000_000

// Buffer wraps a TextBuffer.
type Buffer struct {
	*gtk.TextBuffer
}

// DefaultBuffer returns the default buffer.
func DefaultBuffer() *Buffer {
	defaultOnce.Do(func() { defaultBuffer = NewBuffer() })
	return defaultBuffer
}

// NewBuffer creates a new buffer.
func NewBuffer() *Buffer {
	b := Buffer{}
	b.TextBuffer = gtk.NewTextBuffer(nil)
	b.TextBuffer.SetEnableUndo(false)
	return &b
}

// Write implements io.Writer. It is thread-safe.
func (b *Buffer) Write(bytes []byte) (int, error) {
	glib.IdleAdd(func() {
		endIter := b.EndIter()
		b.Insert(endIter, strings.ToValidUTF8(string(bytes), "\uFFFD"))

		if offset := endIter.Offset(); offset > MaxChars {
			endIter.SetOffset(offset - MaxChars)
			b.Delete(b.StartIter(), endIter)
		}
	})
	return len(bytes), nil
}

// Viewer is a TextView dialog that views a particular log buffer in real time.
type Viewer struct {
	*app.Window
	TextView *gtk.TextView
}

// ShowDefaultViewer calls NewDefaultViewer then Show.
func ShowDefaultViewer(ctx context.Context) {
	NewDefaultViewer(ctx).Show()
}

// NewDefaultViewer creates a new viewer on the default buffer.
func NewDefaultViewer(ctx context.Context) *Viewer {
	return NewViewer(ctx, DefaultBuffer())
}

var _ = cssutil.WriteCSS(`
	.logui-textview {
		margin: 4px 6px;
	}
`)

// NewViewer creates a new log viewer dialog.
func NewViewer(ctx context.Context, buffer *Buffer) *Viewer {
	v := Viewer{}
	v.TextView = gtk.NewTextViewWithBuffer(buffer.TextBuffer)
	v.TextView.AddCSSClass("logui-textview")
	v.TextView.SetEditable(false)
	v.TextView.SetMonospace(true)
	v.TextView.SetVAlign(gtk.AlignEnd)

	scroll := autoscroll.NewWindow()
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetChild(v.TextView)
	scroll.ScrollToBottom()

	v.Window = app.FromContext(ctx).NewWindow()
	v.AddCSSClass("logui-viewer")
	v.SetChild(scroll)
	v.SetTitle(locale.S(ctx, "gotktrix logs"))
	v.SetDefaultSize(500, 400)

	esc := gtk.NewEventControllerKey()
	esc.SetPropagationPhase(gtk.PhaseBubble)
	esc.ConnectKeyPressed(func(val, _ uint, state gdk.ModifierType) bool {
		if val == gdk.KEY_Escape {
			v.Close()
			return true
		}
		return false
	})
	v.AddController(esc)

	return &v
}
