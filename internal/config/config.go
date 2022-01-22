// Package config provides configuration facilities.
package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

// AppID is the prefix for gotktrix's application ID.
const AppID = "com.github.diamondburned.gotktrix"

// AppIDDot creates an AppID path.
func AppIDDot(parts ...string) string {
	if len(parts) == 0 {
		return AppID
	}
	return AppID + "." + strings.Join(parts, ".")
}

var (
	configPath     string
	configPathOnce sync.Once
)

// Path returns the path to the configuration directory with the given tails
// appended. If the path fails, then the function panics.
func Path(tails ...string) string {
	configPathOnce.Do(func() {
		d, err := os.UserConfigDir()
		if err != nil {
			log.Fatalln("failed to get user config dir:", err)
		}

		configPath = filepath.Join(d, "gotktrix")

		// Enforce the right permissions.
		if err := os.MkdirAll(configPath, 0755); err != nil {
			log.Println("error making config dir:", err)
		}
	})

	return joinTails(configPath, tails)
}

var (
	cacheDir     string
	cacheDirOnce sync.Once
)

// CacheDir returns the path to the cache directory of the application.
func CacheDir(tails ...string) string {
	cacheDirOnce.Do(func() {
		d, err := os.UserCacheDir()
		if err != nil {
			d = os.TempDir()
			log.Println("cannot get user cache directory; falling back to", d)
		}

		cacheDir = filepath.Join(d, "gotktrix")

		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			log.Println("error making config dir:", err)
		}
	})

	return joinTails(cacheDir, tails)
}

func joinTails(dir string, tails []string) string {
	if len(tails) == 1 {
		dir = filepath.Join(dir, tails[0])
	} else if len(tails) > 0 {
		paths := append([]string{dir}, tails...)
		dir = filepath.Join(paths...)
	}

	return dir
}

// WriteFile writes b to the file in path atomically.
func WriteFile(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Wrap(err, "cannot mkdir -p")
	}

	tmp, err := os.CreateTemp(dir, ".tmp.*")
	if err != nil {
		return errors.Wrap(err, "cannot mktemp")
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := tmp.Write(b); err != nil {
		return errors.Wrap(err, "cannot write to temp file")
	}
	if err := tmp.Close(); err != nil {
		return errors.Wrap(err, "temp file error")
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.Wrap(err, "cannot swap new prefs file")
	}

	return nil
}
