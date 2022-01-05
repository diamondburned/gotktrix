package imgutil

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/pkg/errors"
)

type opts struct {
	w, h  int
	setFn interface{}
	err   func(error)

	sizer struct {
		set interface {
			SetSizeRequest(w, h int)
			SizeRequest() (w, h int)
		}
		w, h int
	}
}

func (o *opts) error(err error, writeLog bool) {
	if o.err != nil {
		gtkutil.InvokeMain(func() { o.err(err) })
	} else if writeLog {
		log.Println("imgutil:", err)
	}
}

// Opts is a type that can optionally modify the default internal options for
// each call.
type Opts func(*opts)

// OptsError triggers the error handler inside opts if there's one. This is
// useful for asynchronous imgutil function wrappers to signal an error.
func OptsError(opts []Opts, err error) {
	o := processOpts(opts)
	o.error(err, true)
}

func processOpts(optFuncs []Opts) opts {
	var o opts
	for _, opt := range optFuncs {
		opt(&o)
	}
	return o
}

// WithFallbackIcon makes image functions use the icon as the image given into
// the callback instead of a nil one. If name is empty, then dialog-error is
// used. Note that this function overrides WithErrorFn if it is after.
//
// This function only works with AsyncRead and AsyncGET. Using this elsewhere
// will result in a panic.
func WithFallbackIcon(name string) Opts {
	return func(o *opts) {
		o.err = func(error) {
			fn, ok := o.setFn.(func(gdk.Paintabler))
			if !ok {
				return
			}

			w, h := 24, 24
			if o.sizer.w != 0 {
				w = o.sizer.w
			}
			if o.sizer.h != 0 {
				h = o.sizer.h
			}

			icon := IconPaintable(name, w, h)
			fn(icon)
		}
	}
}

// IconPaintable gets the icon with the given name and returns the size. Nil is
// never returned.
func IconPaintable(name string, w, h int) gdk.Paintabler {
	if name == "" {
		name = "image-missing"
	}

	size := w
	if h < w {
		size = h
	}

	theme := gtk.IconThemeGetForDisplay(gdk.DisplayGetDefault())
	if theme == nil {
		panic("imgutil: cannot get IconTheme for default display")
	}

	return theme.LookupIcon(name, nil, size, gtkutil.ScaleFactor(), gtk.TextDirLTR, 0)
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

// WithSizeOverrider overrides the widget's size request to be of the given
// size.
func WithSizeOverrider(widget gtk.Widgetter, w, h int) Opts {
	return func(o *opts) {
		o.sizer.set = gtk.BaseWidget(widget)
		o.sizer.w = w
		o.sizer.h = h
	}
}

// AsyncRead reads the given reader asynchronously into a paintable.
func AsyncRead(ctx context.Context, r io.ReadCloser, f func(gdk.Paintabler), opts ...Opts) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()
		r.Close()
	}()

	o := processOpts(opts)
	o.setFn = f

	do(ctx, &o, true, func() (func(), error) {
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
	var paintable gdk.Paintabler
	var err error

	o := processOpts(opts)
	o.setFn = func(p gdk.Paintabler) { paintable = p }

	p, err := readPixbuf(r, &o)
	if err == nil {
		paintable = gdk.NewTextureForPixbuf(p)
	}

	return paintable, err
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
	o.setFn = f

	do(ctx, &o, true, func() (func(), error) {
		p, err := get(ctx, url, &o)
		if err != nil {
			return nil, err
		}

		return func() { f(p) }, nil
	})
}

// GET gets the given URL into a Paintable.
func GET(ctx context.Context, url string, f func(gdk.Paintabler), opts ...Opts) {
	if url == "" {
		return
	}

	o := processOpts(opts)
	o.setFn = f

	do(ctx, &o, false, func() (func(), error) {
		p, err := get(ctx, url, &o)
		if err != nil {
			return nil, err
		}

		return func() { f(p) }, nil
	})
}

func get(ctx context.Context, url string, o *opts) (gdk.Paintabler, error) {
	pixbuf, err := getPixbuf(ctx, url, o)
	if err != nil {
		return nil, err
	}

	return gdk.NewTextureForPixbuf(pixbuf), nil
}

// AsyncPixbuf fetches a pixbuf.
func AsyncPixbuf(ctx context.Context, url string, f func(*gdkpixbuf.Pixbuf), opts ...Opts) {
	if url == "" {
		return
	}

	o := processOpts(opts)

	do(ctx, &o, true, func() (func(), error) {
		p, err := getPixbuf(ctx, url, &o)
		if err != nil {
			return nil, err
		}

		return func() { f(p) }, nil
	})
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

	r, err := fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p, err := readPixbuf(r, o)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %q", url)
	}

	return p, nil
}

func do(ctx context.Context, o *opts, async bool, do func() (func(), error)) {
	if async {
		go doImpl(ctx, o, do)
	} else {
		doImpl(ctx, o, do)
	}
}

func doImpl(ctx context.Context, o *opts, do func() (func(), error)) {
	f, err := do()
	if err != nil {
		o.error(err, true)
		return
	}

	glib.IdleAdd(func() {
		select {
		case <-ctx.Done():
			// don't call f if cancelledd
			o.error(ctx.Err(), false)
		default:
			f()
		}
	})
}

var errNilPixbuf = errors.New("nil pixbuf")

func readPixbuf(r io.Reader, opts *opts) (*gdkpixbuf.Pixbuf, error) {
	loader := gdkpixbuf.NewPixbufLoader()
	loader.Connect("size-prepared", func(loader *gdkpixbuf.PixbufLoader, w, h int) {
		if opts.w > 0 && opts.h > 0 {
			if w != opts.w || h != opts.h {
				w, h = gotktrix.MaxSize(w, h, opts.w, opts.h)
				loader.SetSize(w, h)
			}
		}
		if opts.sizer.set != nil {
			maxW, maxH := opts.sizer.w, opts.sizer.h
			if maxW == 0 && maxH == 0 {
				maxW, maxH = opts.sizer.set.SizeRequest()
			}
			if maxW == 0 && maxH == 0 {
				maxW, maxH = opts.w, opts.h
			}
			opts.sizer.set.SetSizeRequest(gotktrix.MaxSize(w, h, maxW, maxH))
		}
	})

	if err := pixbufLoaderReadFrom(loader, r); err != nil {
		return nil, errors.Wrap(err, "reader error")
	}

	pixbuf := loader.Pixbuf()
	if pixbuf == nil {
		return nil, errNilPixbuf
	}

	return pixbuf, nil
}

const defaultBufsz = 1 << 17 // 128KB

var bufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, defaultBufsz)
		return &b
	},
}

func pixbufLoaderReadFrom(l *gdkpixbuf.PixbufLoader, r io.Reader) error {
	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)

	_, err := io.CopyBuffer(gioutil.PixbufLoaderWriter(l), r, *buf)
	if err != nil {
		l.Close()
		return err
	}

	if err := l.Close(); err != nil {
		return fmt.Errorf("failed to close PixbufLoader: %w", err)
	}

	return nil
}

// MaxSize returns the maximum size that can fit within the given max width and
// height. Aspect ratio is preserved.
func MaxSize(w int, h int, maxW int, maxH int) (int, int) {
	return gotktrix.MaxSize(w, h, maxW, maxH)
}
