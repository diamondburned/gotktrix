package blinker

import (
	"context"

	"github.com/chanbakjsd/gotrix/api"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

type blinkerState uint8

const (
	blinkerNone blinkerState = iota
	blinkerSync
	blinkerError
)

const (
	// blinkerSyncIcon  = "emblem-favorite-symbolic"
	// blinkerErrorIcon = "dialog-error-symbolic"
	blinkerStayTime = 200 // ms
)

func (s blinkerState) Icon() string {
	switch s {
	case blinkerSync:
		return "emblem-favorite-symbolic"
	case blinkerError:
		return "dialog-error-symbolic"
	default:
		return ""
	}
}

func (s blinkerState) Class() string {
	switch s {
	case blinkerSync:
		return "blinker-sync"
	case blinkerError:
		return "blinker-error"
	}
	return ""
}

type blinker struct {
	gtk.Image
	prev  glib.SourceHandle
	state blinkerState
}

var blinkerCSS = cssutil.Applier("blinker", `
	@define-color blinker-heart-color #F7A8B8;

	.blinker {
		color:   @blinker-heart-color;
		margin:  6px;
		opacity: 0;
		transition: linear 650ms;
		transition-property: opacity, color;
	}
	.blinker-sync {
		opacity: 0.8;
		transition: none;
	}
	.blinker-error {
		opacity: 1;
		color: red;
	}
`)

// New creates a new blinker.
func New(ctx context.Context) gtk.Widgetter {
	img := gtk.NewImageFromIconName("content-loading-symbolic")
	img.SetIconSize(gtk.IconSizeNormal)
	blinkerCSS(img)

	b := &blinker{Image: *img}

	client := gotktrix.FromContext(ctx)

	var funcs []func()
	b.ConnectMap(func() {
		funcs = []func(){
			client.OnSync(func(*api.SyncResponse) {
				glib.IdleAdd(func() { b.sync() })
			}),
			client.OnSyncError(func(err error) {
				glib.IdleAdd(func() { b.error(err) })
			}),
		}
	})
	b.ConnectUnmap(func() {
		for _, fn := range funcs {
			fn()
		}
	})

	return b
}

func (b *blinker) sync() {
	b.set(blinkerSync)

	if b.prev != 0 {
		glib.SourceRemove(b.prev)
	}

	b.prev = glib.TimeoutAdd(blinkerStayTime, func() {
		b.set(blinkerNone)
		b.prev = 0
	})
}

func (b *blinker) error(err error) {
	b.set(blinkerError)
	b.SetTooltipText(err.Error())
}

func (b *blinker) set(state blinkerState) {
	b.SetTooltipText("")

	if b.state != blinkerNone {
		b.RemoveCSSClass(b.state.Class())
		b.state = blinkerNone
	}

	b.state = state

	if class := state.Class(); class != "" {
		b.AddCSSClass(class)
	}

	if icon := state.Icon(); icon != "" {
		b.SetFromIconName(icon)
	}
}

/*
func (b *blinker) cas(state, ifState blinkerState) {
	if b.state != ifState {
		return
	}

	b.RemoveCSSClass(ifState.Class())
	b.state = blinkerNone

	b.set(state)
}
*/
