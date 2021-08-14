package imgutil

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"time"

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

// AsyncGET GETs the given URL and calls f in the main loop. If the context is
// cancelled by the time GET is done, then f will not be called. If the given
// URL is nil, then the function does nothing.
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

		f, err := do()
		if err != nil {
			log.Println("async GET error:", err)
			return
		}

		glib.IdleAdd(func() {
			// Don't release until the callback is done.
			defer sema.Release(1)

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

func GETPixbuf(ctx context.Context, url string) (*gdkpixbuf.Pixbuf, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request %q", url)
	}

	r, err := Client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to GET %q", url)
	}
	defer r.Body.Close()

	loader := gdkpixbuf.NewPixbufLoader()
	if err := pixbufLoaderReadFrom(loader, r.Body); err != nil {
		return nil, errors.Wrapf(err, "failed to read response %q", url)
	}

	pixbuf := loader.Pixbuf()
	if pixbuf == nil {
		return nil, fmt.Errorf("no pixbuf rendered for %q", url)
	}

	return pixbuf, nil
}

type pixbufLoaderWriter gdkpixbuf.PixbufLoader

func pixbufLoaderReadFrom(l *gdkpixbuf.PixbufLoader, r io.Reader) error {
	_, err := io.Copy((*pixbufLoaderWriter)(l), r)
	if err != nil {
		l.Close()
		return err
	}
	if err := l.Close(); err != nil {
		return fmt.Errorf("failed to close PixbufLoader: %w", err)
	}
	return nil
}

func (w *pixbufLoaderWriter) Write(b []byte) (int, error) {
	if err := (*gdkpixbuf.PixbufLoader)(w).Write(b); err != nil {
		return 0, err
	}
	return len(b), nil
}
