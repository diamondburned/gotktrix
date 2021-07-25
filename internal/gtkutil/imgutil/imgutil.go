package imgutil

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/gotk3/gotk3/glib"
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

	go func() {
		if err := sema.Acquire(ctx, 1); err != nil {
			return
		}

		p, err := GET(ctx, url)
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
				f(p)
			}
		})
	}()
}

// GET gets the given URL into a Paintable.
func GET(ctx context.Context, url string) (gdk.Paintabler, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	r, err := Client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to GET")
	}
	defer r.Body.Close()

	mediaFile := gtk.NewMediaFile()

	stream, err := newInputStream(r.Body, mediaFile.GError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make fd stream")
	}

	readFile := stream.readFile

	mediaFile.SetInputStream(stream)
	mediaFile.Connect("notify::ended", func() {
		// Keep the fd alive until the media stream is done.
		runtime.KeepAlive(readFile)
	})

	return mediaFile, nil
}

type inputStream struct {
	*gio.UnixInputStream
	readFile *os.File
}

var bytePool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 2<<20) // 2MB buffer
	},
}

func newInputStream(r io.Reader, onErr func(err error)) (inputStream, error) {
	rp, wp, err := os.Pipe()
	if err != nil {
		return inputStream{}, errors.Wrap(err, "failed to mkpipe")
	}

	go func() {
		defer wp.Close()

		buffer := bytePool.Get().([]byte)
		defer bytePool.Put(buffer)

		_, err := wp.ReadFrom(r)
		if err != nil {
			onErr(errors.Wrap(err, "error copying to gio.UnixInputStream"))
		}
	}()

	return inputStream{
		UnixInputStream: gio.NewUnixInputStream(int(rp.Fd()), true),
		readFile:        rp,
	}, nil
}
