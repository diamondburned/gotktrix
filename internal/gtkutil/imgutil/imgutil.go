package imgutil

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

// Client is the HTTP client used to fetch all images.
var Client = http.Client{
	Timeout: 15 * time.Second,
	Transport: httpcache.NewTransport(
		diskcache.New(config.CacheDir("img")),
	),
}

// parallelMult * 4 = maxConcurrency
const parallelMult = 4

// sema is used to throttle concurrent downloads.
var sema = semaphore.NewWeighted(int64(runtime.GOMAXPROCS(-1)) * parallelMult)

// AsyncRead reads the given reader asynchronously into a paintable.
func AsyncRead(ctx context.Context, r io.ReadCloser, f func(gdk.Paintabler)) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()
		r.Close()
	}()

	async(ctx, func() (func(), error) {
		defer cancel()

		p, err := Read(r)
		if err != nil {
			return nil, err
		}

		return func() { f(p) }, nil
	})
}

// Read synchronously reads the reader into a paintable.
func Read(r io.Reader) (gdk.Paintabler, error) {
	p, err := readPixbuf(r)
	if err != nil {
		return nil, err
	}

	return gdk.NewTextureForPixbuf(p), nil
}

// AsyncGET GETs the given URL and calls f in the main loop. If the context is
// cancelled by the time GET is done, then f will not be called. If the given
// URL is nil, then the function does nothing.
//
// This function can be called from any thread. It will synchronize accordingly
// by itself.
func AsyncGET(ctx context.Context, url string, f func(gdk.Paintabler)) {
	if url == "" {
		return
	}

	async(ctx, func() (func(), error) {
		p, err := GET(ctx, url)
		if err != nil {
			return nil, errors.Wrap(err, "async GET error")
		}

		return func() { f(p) }, nil
	})
}

// AsyncPixbuf fetches a pixbuf.
func AsyncPixbuf(ctx context.Context, url string, f func(*gdkpixbuf.Pixbuf)) {
	if url == "" {
		return
	}

	async(ctx, func() (func(), error) {
		p, err := GETPixbuf(ctx, url)
		if err != nil {
			return nil, errors.Wrap(err, "async GET error")
		}

		return func() { f(p) }, nil
	})
}

func async(ctx context.Context, do func() (func(), error)) {
	go func() {
		if err := sema.Acquire(ctx, 1); err != nil {
			return
		}
		defer sema.Release(1)

		f, err := do()
		if err != nil {
			log.Println("imgutil GET:", err)
			return
		}

		glib.IdleAdd(func() {
			select {
			case <-ctx.Done():
				// don't call f if cancelledd
			default:
				f()
			}
		})
	}()
}

// GET gets the given URL into a Paintable.
func GET(ctx context.Context, url string) (gdk.Paintabler, error) {
	pixbuf, err := GETPixbuf(ctx, url)
	if err != nil {
		return nil, err
	}

	return gdk.NewTextureForPixbuf(pixbuf), nil
}

// GETPixbuf gets the Pixbuf directly.
func GETPixbuf(ctx context.Context, url string) (*gdkpixbuf.Pixbuf, error) {
	if url == "" {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request %q", url)
	}

	r, err := Client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to GET %q", url)
	}
	defer r.Body.Close()

	p, err := readPixbuf(r.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %q", url)
	}

	return p, nil
}

func readPixbuf(r io.Reader) (*gdkpixbuf.Pixbuf, error) {
	loader := gdkpixbuf.NewPixbufLoader()
	if err := pixbufLoaderReadFrom(loader, r); err != nil {
		return nil, errors.Wrap(err, "reader error")
	}

	pixbuf := loader.Pixbuf()
	if pixbuf == nil {
		return nil, errors.New("nil pixbuf")
	}

	return pixbuf, nil
}

func pixbufLoaderReadFrom(l *gdkpixbuf.PixbufLoader, r io.Reader) error {
	_, err := io.Copy(gioutil.PixbufLoaderWriter(l), r)
	if err != nil {
		l.Close()
		return err
	}
	if err := l.Close(); err != nil {
		return fmt.Errorf("failed to close PixbufLoader: %w", err)
	}
	return nil
}
