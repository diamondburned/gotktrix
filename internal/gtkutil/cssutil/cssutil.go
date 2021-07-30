package cssutil

import (
	"bytes"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/config"
)

var globalCSS bytes.Buffer

// Applier returns a constructor that applies a class to the given widgetter. It
// also writes the CSS to the global CSS.
func Applier(class, css string) func(gtk.Widgetter) {
	globalCSS.WriteString(css)
	classes := strings.Split(class, ".")
	return func(w gtk.Widgetter) {
		ctx := w.StyleContext()
		for _, class := range classes {
			ctx.AddClass(class)
		}
	}
}

var (
	userCSS  []byte
	userOnce sync.Once
)

// ApplyGlobalCSS applies the current global CSS to the default display.
func ApplyGlobalCSS() {
	userOnce.Do(func() {
		f, err := os.ReadFile(config.Path("user.css"))
		if err != nil {
			log.Println("failed to read user.css:", err)
			return
		}

		userCSS = f
	})

	prov := gtk.NewCSSProvider()
	prov.LoadFromData(globalCSS.Bytes())

	display := gdk.DisplayGetDefault()
	gtk.StyleContextAddProviderForDisplay(display, prov, 600) // app

	if userCSS != nil {
		prov := gtk.NewCSSProvider()
		prov.LoadFromData(userCSS)

		gtk.StyleContextAddProviderForDisplay(display, prov, 800) // user
	}
}
