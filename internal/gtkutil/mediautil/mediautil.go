package mediautil

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var defaultClient = http.Client{
	Timeout: 4 * time.Minute,
}

const gcPeriod = time.Minute

var (
	gcs  = map[string]*cacheGC{}
	gcMu sync.Mutex
)

func doGC(path string, age time.Duration) {
	gcMu.Lock()

	gc, ok := gcs[path]
	if !ok {
		gc = &cacheGC{}
		gcs[path] = gc
	}

	gcMu.Unlock()

	gc.do(path, age)
}

type cacheGC struct {
	mut     sync.Mutex
	lastRun time.Time
	running bool
}

// do runs the GC asynchronously.
func (c *cacheGC) do(path string, age time.Duration) {
	now := time.Now()

	// Only run the GC after the set period and once the previous GC job is
	// done.
	c.mut.Lock()
	if c.running || c.lastRun.Add(gcPeriod).After(now) {
		c.mut.Unlock()
		return
	}
	c.running = true
	c.lastRun = now
	c.mut.Unlock()

	go func() {
		files, _ := os.ReadDir(path)

		for _, file := range files {
			s, err := file.Info()
			if err != nil {
				continue
			}

			if s.ModTime().Add(age).Before(now) {
				// Outdated.
				os.Remove(filepath.Join(path, file.Name()))
			}
		}

		c.mut.Lock()
		c.running = false
		c.mut.Unlock()
	}()
}

// isFile returns true if the given path exists as a file.
func isFile(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

// doTmp gives f a premade tmp file and moves it back atomically.
func doTmp(dst, pattern string, fn func(path string) error) (string, error) {
	if isFile(dst) {
		return dst, nil
	}

	dir := filepath.Dir(dst)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", errors.Wrap(err, "cannot mkdir -p")
	}

	f, err := os.CreateTemp(dir, ".tmp."+pattern)
	if err != nil {
		return "", errors.Wrap(err, "cannot mktemp")
	}
	f.Close()

	defer os.Remove(f.Name())

	if err := fn(f.Name()); err != nil {
		return "", err
	}

	if err := os.Rename(f.Name(), dst); err != nil {
		return "", errors.Wrap(err, "cannot rename tmp file")
	}

	return dst, nil
}
