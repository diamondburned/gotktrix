package imgutil

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
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

type opts struct {
	w, h int
	err  func(error)
}

func (o *opts) error(err error) {
	if o.err != nil {
		o.err(err)
	}
}

// Opts is a type that can optionally modify the default internal options for
// each call.
type Opts func(*opts)

func processOpts(optFuncs []Opts) opts {
	var o opts
	for _, opt := range optFuncs {
		opt(&o)
	}
	return o
}

// WithErrorFn adds a callback that is called on an error.
func WithErrorFn(f func(error)) Opts {
	return func(o *opts) { o.err = f }
}

// WithRectRescale is a convenient function around WithRescale for rectangular
// or circular images.
func WithRectRescale(size int) Opts {
	return WithRescale(size, size)
}

// WithRescale rescales the image to the given max width and height while
// respecting its aspect ratio. The given sizes will be used as the maximum
// sizes.
func WithRescale(w, h int) Opts {
	return func(o *opts) { o.w, o.h = w, h }
}

// AsyncRead reads the given reader asynchronously into a paintable.
func AsyncRead(ctx context.Context, r io.ReadCloser, f func(gdk.Paintabler), opts ...Opts) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()
		r.Close()
	}()

	o := processOpts(opts)

	async(ctx, &o, func() (func(), error) {
		defer cancel()

		p, err := Read(r)
		if err != nil {
			return nil, err
		}

		return func() { f(p) }, nil
	})
}

// Read synchronously reads the reader into a paintable.
func Read(r io.Reader, opts ...Opts) (gdk.Paintabler, error) {
	o := processOpts(opts)

	p, err := readPixbuf(r, &o)
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
func AsyncGET(ctx context.Context, url string, f func(gdk.Paintabler), opts ...Opts) {
	if url == "" {
		return
	}

	o := processOpts(opts)

	async(ctx, &o, func() (func(), error) {
		p, err := get(ctx, url, &o)
		if err != nil {
			return nil, errors.Wrap(err, "async GET error")
		}

		return func() { f(p) }, nil
	})
}

// AsyncPixbuf fetches a pixbuf.
func AsyncPixbuf(ctx context.Context, url string, f func(*gdkpixbuf.Pixbuf), opts ...Opts) {
	if url == "" {
		return
	}

	o := processOpts(opts)

	async(ctx, &o, func() (func(), error) {
		p, err := getPixbuf(ctx, url, &o)
		if err != nil {
			return nil, errors.Wrap(err, "async GET error")
		}

		return func() { f(p) }, nil
	})
}

func async(ctx context.Context, o *opts, do func() (func(), error)) {
	go func() {
		if err := sema.Acquire(ctx, 1); err != nil {
			err = errors.Wrap(err, "failed to acquire ctx")
			glib.IdleAdd(func() { o.error(err) })
			return
		}
		defer sema.Release(1)

		f, err := do()
		if err != nil {
			if o.err != nil {
				glib.IdleAdd(func() { o.err(err) })
			} else {
				log.Println("imgutil GET:", err)
			}
			return
		}

		glib.IdleAdd(func() {
			select {
			case <-ctx.Done():
				// don't call f if cancelledd
				o.error(ctx.Err())
			default:
				f()
			}
		})
	}()
}

// GET gets the given URL into a Paintable.
func GET(ctx context.Context, url string, opts ...Opts) (gdk.Paintabler, error) {
	o := processOpts(opts)
	return get(ctx, url, &o)
}

func get(ctx context.Context, url string, o *opts) (gdk.Paintabler, error) {
	pixbuf, err := getPixbuf(ctx, url, o)
	if err != nil {
		return nil, err
	}

	return gdk.NewTextureForPixbuf(pixbuf), nil
}

// GETPixbuf gets the Pixbuf directly.
func GETPixbuf(ctx context.Context, url string, opts ...Opts) (*gdkpixbuf.Pixbuf, error) {
	o := processOpts(opts)
	return getPixbuf(ctx, url, &o)
}

func getPixbuf(ctx context.Context, url string, o *opts) (*gdkpixbuf.Pixbuf, error) {
	if url == "" {
		return nil, errors.New("empty URL given")
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

	if r.StatusCode < 200 || r.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected status code %d getting %q", r.StatusCode, url)
	}

	p, err := readPixbuf(r.Body, o)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %q", url)
	}

	return p, nil
}

func readPixbuf(r io.Reader, opts *opts) (*gdkpixbuf.Pixbuf, error) {
	loader := gdkpixbuf.NewPixbufLoader()
	if opts.w > 0 && opts.h > 0 {
		loader.Connect("size-prepared", func(loader *gdkpixbuf.PixbufLoader, w, h int) {
			if w != opts.w || h != opts.h {
				loader.SetSize(gotktrix.MaxSize(w, h, opts.w, opts.h))
			}
		})
	}

	if err := pixbufLoaderReadFrom(loader, r); err != nil {
		return nil, errors.Wrap(err, "reader error")
	}

	pixbuf := loader.Pixbuf()
	if pixbuf == nil {
		return nil, errors.New("nil pixbuf")
	}

	return pixbuf, nil
}

const defaultBufsz = 1 << 17 // 128KB

var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, defaultBufsz)
	},
}

func pixbufLoaderReadFrom(l *gdkpixbuf.PixbufLoader, r io.Reader) error {
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)

	_, err := io.CopyBuffer(gioutil.PixbufLoaderWriter(l), r, buf)
	if err != nil {
		l.Close()
		return err
	}

	if err := l.Close(); err != nil {
		return fmt.Errorf("failed to close PixbufLoader: %w", err)
	}

	return nil
}
