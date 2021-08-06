package aggressivegc

import (
	"runtime"
	"time"
)

func init() {
	go func() {
		for range time.Tick(5 * time.Second) {
			runtime.GC()
		}
	}()
}
