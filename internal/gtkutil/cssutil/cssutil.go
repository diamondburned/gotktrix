package cssutil

import (
	"bytes"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
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

// ApplyGlobalCSS applies the current global CSS to the default display.
func ApplyGlobalCSS() {
	prov := gtk.NewCSSProvider()
	prov.LoadFromData(globalCSS.Bytes())

	display := gdk.DisplayGetDefault()
	gtk.StyleContextAddProviderForDisplay(display, prov, 600)
}
