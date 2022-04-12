package gotktrix

import (
	"log"
	"os"
	"sync"

	"github.com/diamondburned/gotrix/debug"
)

var (
	// InfoEnabled is true if GOMATRIX_INFO is non-empty.
	InfoEnabled bool
	// WarnEnabled is true if GOMATRIX_WARN is non-empty.
	WarnEnabled bool
)

var (
	logger   = log.New(os.Stderr, "gotrix: ", log.LstdFlags)
	initOnce sync.Once
)

func init() {
	debug.Logger = matrixLogger{}
}

// logInit is called on every wrapClient.
func logInit() {
	initOnce.Do(func() {
		InfoEnabled = os.Getenv("GOMATRIX_INFO") != ""
		InfoEnabled = InfoEnabled || debug.DebugEnabled

		WarnEnabled = os.Getenv("GOMATRIX_WARN") != ""
		WarnEnabled = WarnEnabled || InfoEnabled
	})
}

type matrixLogger struct{}

func (matrixLogger) Trace(a interface{}) {
	if debug.TraceEnabled {
		logger.Println("trace:", a)
	}
}

func (matrixLogger) Debug(a interface{}) {
	if debug.DebugEnabled {
		logger.Println("debug:", a)
	}
}

func (matrixLogger) Info(a interface{}) {
	if InfoEnabled {
		logger.Println("info:", a)
	}
}

func (matrixLogger) Warn(a interface{}) {
	if WarnEnabled {
		logger.Println("warn:", a)
	}
}

func (matrixLogger) Error(a interface{}) {
	logger.Println("error:", a)
}
