// Package config provides configuration facilities.
package config

import (
	"log"
	"os"
	"path/filepath"
	"sync"
)

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

		configPath = d
	})

	d := configPath

	if len(tails) == 1 {
		d = filepath.Join(d, tails[0])
	} else if len(tails) > 0 {
		paths := append([]string{d}, tails...)
		d = filepath.Join(paths...)
	}

	return d
}
