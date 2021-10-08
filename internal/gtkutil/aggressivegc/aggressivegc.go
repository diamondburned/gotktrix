package aggressivegc

import (
	"runtime"
	"time"
)

func init() {
	go func() {
		for range time.Tick(30 * time.Second) {
			runtime.GC()
		}
	}()
}
