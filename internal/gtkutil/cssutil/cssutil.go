package cssutil

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func Applier(class, css string) func(gtk.Widgetter) {
	prov := gtk.NewCSSProvider()
	// TODO: CSS error
	prov.LoadFromData([]byte(css))

	classes := strings.Split(class, ".")

	return func(w gtk.Widgetter) {
		ctx := w.StyleContext()
		ctx.AddProvider(prov, 600) // Application

		for _, class := range classes {
			ctx.AddClass(class)
		}
	}
}
