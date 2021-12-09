package aggressivegc

import (
	"runtime"
	"time"
)

func init() {
	go func() {
		for range time.Tick(time.Minute) {
			runtime.GC()
		}
	}()
}
