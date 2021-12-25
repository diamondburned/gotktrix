package progress

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/pkg/errors"
)

// Reader wraps a progress bar and implements io.Reader.
type Reader struct {
	r io.Reader
	b *Bar

	n      int64
	handle glib.SourceHandle
}

// WrapReader wraps the given io.Reader to also update the given Bar.
func WrapReader(r io.Reader, b *Bar) *Reader {
	return &Reader{r: r, b: b}
}

// Bar returns the reader's bar.
func (r *Reader) Bar() *Bar { return r.b }

const updateFreq = 1000 / 20 // 20Hz

// Read implements io.Reader.
func (r *Reader) Read(b []byte) (int, error) {
	if r.handle == 0 {
		r.handle = glib.TimeoutAddPriority(updateFreq, glib.PriorityDefaultIdle, func() bool {
			r.update()
			return true
		})
	}

	n, err := r.r.Read(b)
	atomic.AddInt64(&r.n, int64(n))

	if err != nil {
		glib.SourceRemove(r.handle)
		r.handle = 0
		// Ensure that the state gets updated one last time.
		glib.IdleAdd(r.update)
	}

	return n, err
}

func (r *Reader) update() {
	r.b.Set(atomic.LoadInt64(&r.n))
}

// Download is a helper function that downloads the resource at the given URL
// into the path at the given dst. The download progress is shown in the bar.
func Download(ctx context.Context, url, dst string, b *Bar) error {
	if err := download(ctx, url, dst, b); err != nil {
		glib.IdleAdd(func() { b.Error(err) })
		return err
	}
	return nil
}

func download(ctx context.Context, url, dst string, b *Bar) error {
	dstDir := filepath.Dir(dst)

	f, err := os.CreateTemp(dstDir, ".tmp.*")
	if err != nil {
		return errors.Wrap(err, "cannot mktemp")
	}
	defer os.Remove(f.Name())
	defer f.Close()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(f, WrapReader(resp.Body, b)); err != nil {
		return err
	}

	resp.Body.Close()
	if err := f.Close(); err != nil {
		return errors.Wrap(err, "cannot close file")
	}

	if err := os.Rename(f.Name(), dst); err != nil {
		return errors.Wrap(err, "cannot mv downloaded file")
	}

	return nil
}
