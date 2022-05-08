package uploadutil

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/matrix"
)

var progressBarCSS = cssutil.Applier("uploadutil-progress", `
	.uploadutil-progress.failed text {
		color: red;
	}
	.uploadutil-progress.failed progress {
		background-color: red;
		border-color: red;
		color: red;
	}
`)

// ProgressBar is a wrapper around gtk.ProgressBar.
type ProgressBar struct {
	*gtk.ProgressBar

	total int64
	old   int64
	new   int64 // atomic

	updater glib.SourceHandle
}

func NewProgressBar() *ProgressBar {
	p := &ProgressBar{
		ProgressBar: gtk.NewProgressBar(),

		new:   0,
		old:   0,
		total: 0,
	}

	p.SetPulseStep(0.01) // 1%
	p.Reset()

	progressBarCSS(p)
	return p
}

// Reset resets the progress bar.
func (p *ProgressBar) Reset() {
	p.new = 0
	p.old = 0
	p.total = 0
	p.SetFraction(0)
	p.RemoveCSSClass("failed")

	if p.updater != 0 {
		glib.SourceRemove(p.updater)
	}

	p.updater = glib.TimeoutAdd(1000/15, func() bool {
		new := atomic.LoadInt64(&p.new)
		if new == 0 {
			// Don't update if no read yet.
			return true
		}

		if total := atomic.LoadInt64(&p.total); total == 0 {
			if new != p.old {
				p.Pulse()
				p.old = new
			}
		} else {
			p.SetFraction(float64(new) / float64(total))
		}

		return true
	})
}

// SetTotal sets the total number of bytes to upload. This method is safe to use
// concurrently.
func (p *ProgressBar) SetTotal(bytes int64) {
	atomic.StoreInt64(&p.total, bytes)
}

// Read increments the bytes read. This method is safe to use concurrently.
func (p *ProgressBar) Read(bytes int64) {
	atomic.AddInt64(&p.new, bytes)
}

// Done marks the progress bar as done. This method is safe to use concurrently.
func (p *ProgressBar) Done(err bool) {
	glib.IdleAdd(func() {
		if p.updater == 0 {
			return
		}

		// Make sure only the first Done run counts.
		glib.SourceRemove(p.updater)
		p.updater = 0

		if err {
			p.AddCSSClass("failed")
			p.SetText("Error")
		}

		p.SetFraction(1)
	})
}

// Error sets an error.
func (p *ProgressBar) Error() {
	if p.updater != 0 {
		glib.SourceRemove(p.updater)
		p.updater = 0
	}

	p.AddCSSClass("failed")
	p.SetText("Error")
}

// progressReader wraps around a ProgressBar to increment it using the io.Reader
// API.
type progressReader struct {
	b *ProgressBar
	r io.ReadCloser
}

// WrapProgressReader wraps a ProgressBar to update it when the reader is read
// from.
func WrapProgressReader(b *ProgressBar, r io.ReadCloser) io.ReadCloser {
	return progressReader{b, r}
}

func (p progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if err != nil {
		p.b.Done(!errors.Is(err, io.EOF))
	} else {
		p.b.Read(int64(n))
	}
	return n, err
}

func (p progressReader) Close() error {
	err := p.r.Close()
	p.b.Done(err != nil)
	return err
}

// WrapCloser wraps a reader around a closer.
func WrapCloser(r io.Reader, c io.Closer) io.ReadCloser {
	return readCloser{
		Reader: r,
		Closer: c,
	}
}

type readCloser struct {
	io.Reader
	io.Closer
}

const bufferSize = 1 << 15 // 32KB

// Upload wraps around a reader to peek for its MIME type.
func Upload(c *gotktrix.Client, r io.ReadCloser, name string) (matrix.URL, error) {
	buf := bufio.NewReaderSize(r, bufferSize)

	b, err := buf.Peek(512)
	if err != nil {
		return "", err
	}

	return c.MediaUpload(http.DetectContentType(b), name, WrapCloser(buf, r))
}
