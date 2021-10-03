package cursed

/*
#cgo pkg-config: glib-2.0 gobject-introspection-1.0 gtk4 libadwaita-1
#cgo CFLAGS: -Wno-deprecated-declarations
#include <gtk/gtk.h>
#include <adwaita.h>
*/
import "C"

import (
	"runtime"
	"unsafe"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type _StyleManager struct {
	_        C.GObject
	display  uintptr
	settings uintptr
	provider unsafe.Pointer
}

func StyleManagerProvider(manager *adw.StyleManager) *gtk.CSSProvider {
	csm := (*_StyleManager)(unsafe.Pointer(manager.Native()))
	provider := glib.Take(csm.provider)
	runtime.KeepAlive(manager)

	return &gtk.CSSProvider{
		Object: provider,
		StyleProvider: gtk.StyleProvider{
			Object: provider,
		},
	}
}
