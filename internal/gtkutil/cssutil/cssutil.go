package cssutil

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/config"
)

var globalCSS = func() *strings.Builder {
	b := strings.Builder{}
	b.Grow(50 << 10) // 50KB
	return &b
}()

// Applier returns a constructor that applies a class to the given widgetter. It
// also writes the CSS to the global CSS.
func Applier(class, css string) func(gtk.Widgetter) {
	globalCSS.WriteString(css)
	classes := strings.Split(class, ".")
	return func(w gtk.Widgetter) {
		for _, class := range classes {
			gtk.BaseWidget(w).AddCSSClass(class)
		}
	}
}

// WriteCSS adds the given string to the global CSS. It's primarily meant to be
// used during global variable initialization.
func WriteCSS(css string) struct{} {
	globalCSS.WriteString(css)
	return struct{}{}
}

var _ = WriteCSS(`
	avatar { border-radius: 999px; }
`)

// AddClass adds classes.
func AddClass(w gtk.Widgetter, classes ...string) {
	ctx := gtk.BaseWidget(w).StyleContext()
	for _, class := range classes {
		ctx.AddClass(class)
	}
}

var (
	userCSS  string
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

		userCSS = string(f)
	})

	css := globalCSS.String()

	prov := gtk.NewCSSProvider()
	prov.Connect("parsing-error", func(sec *gtk.CSSSection, err error) {
		loc := sec.StartLocation()

		lines := strings.Split(css, "\n")
		log.Printf("CSS error (%v) at line: %q", err, lines[loc.Lines()])
	})

	prov.LoadFromData(css)

	display := gdk.DisplayGetDefault()
	gtk.StyleContextAddProviderForDisplay(display, prov, 600) // app

	if userCSS != "" {
		prov := gtk.NewCSSProvider()
		prov.LoadFromData(userCSS)

		gtk.StyleContextAddProviderForDisplay(display, prov, 800) // user
	}
}
