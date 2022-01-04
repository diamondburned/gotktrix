package imgutil

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

// Client is the HTTP client used to fetch all images.
var Client = http.Client{
	Timeout: 15 * time.Second,
	Transport: &httpcache.Transport{
		Cache: diskcache.New(config.CacheDir("img")),
	},
}

// parallelMult * 4 = maxConcurrency
const parallelMult = 4

// parallel is used to throttle concurrent downloads.
var parallel = semaphore.NewWeighted(int64(runtime.GOMAXPROCS(-1)) * parallelMult)

var (
	fetchingURLs = map[string]*sync.Mutex{}
	fetchingMu   sync.Mutex

	// TODO: limit the size of invalidURLs.
	invalidURLs sync.Map
)

var errURLNotFound = errors.New("URL not found (cached)")

func urlIsInvalid(url string) bool {
	h := hashURL(url)

	vt, ok := invalidURLs.Load(h)
	if !ok {
		return false
	}

	t := time.Unix(vt.(int64), 0)
	if t.Add(time.Hour).After(time.Now()) {
		// fetched within the hour
		return true
	}

	invalidURLs.Delete(h)
	return false
}

func markURLInvalid(url string) {
	invalidURLs.Store(hashURL(url), time.Now().Unix())
}

// hashURL ensures that keys in the invalidURLs map are always 24 bytes long to
// reduce the length that each key takes. This puts additional (but minimal)
// strain on the GC.
func hashURL(url string) [sha256.Size224]byte {
	return sha256.Sum224([]byte(url))
}

func fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	if urlIsInvalid(url) {
		return nil, errURLNotFound
	}

	// How this works: we acquire a mutex for each request so that only 1
	// request per URL is ever sent. We will then perform the request so that
	// the cache is populated, and then repeat. This way, only 1 parallel
	// request per URL is ever done, but the ratio of cache hits is much higher.
	//
	// This isn't too bad, actually. Only the initial HTTP connection is done on
	// its own; the images will still be downloaded in parallel.

	fetchingMu.Lock()
	urlMut, ok := fetchingURLs[url]
	if !ok {
		urlMut = &sync.Mutex{}
		fetchingURLs[url] = urlMut
	}
	fetchingMu.Unlock()

	defer func() {
		fetchingMu.Lock()
		delete(fetchingURLs, url)
		fetchingMu.Unlock()
	}()

	urlMut.Lock()
	defer urlMut.Unlock()

	// Recheck with the acquired lock.
	if urlIsInvalid(url) {
		return nil, errURLNotFound
	}

	// Only acquire the semaphore once we've acquired the per-URL mutex, just to
	// ensure that all n different URLs can run in paralle.
	if err := parallel.Acquire(ctx, 1); err != nil {
		return nil, errors.Wrap(err, "failed to acquire ctx")
	}
	defer parallel.Release(1)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request %q", url)
	}

	r, err := Client.Do(req)
	if err != nil {
		return nil, err
	}

	if r.StatusCode < 200 || r.StatusCode > 299 {
		if r.StatusCode >= 400 && r.StatusCode <= 499 {
			markURLInvalid(url)
		}

		r.Body.Close()
		return nil, fmt.Errorf("unexpected status code %d getting %q", r.StatusCode, url)
	}

	return r.Body, nil
}
