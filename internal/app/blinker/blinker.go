package blinker

import (
	"context"
	"net/http"

	"github.com/chanbakjsd/gotrix/api"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

type blinkerState uint8

const (
	blinkerNone blinkerState = iota
	blinkerSync
	blinkerSyncing
	blinkerDownloading
	blinkerError
)

const blinkerStayTime = 200 // ms

func (s blinkerState) Icon() string {
	switch s {
	case blinkerSync:
		return "emblem-favorite-symbolic"
	case blinkerSyncing:
		return "emblem-synchronizing-symbolic" // or "network-idle-symbolic"
	case blinkerDownloading:
		return "network-receive-symbolic"
	case blinkerError:
		return "network-error-symbolic"
	default:
		return ""
	}
}

func (s blinkerState) Class() string {
	switch s {
	case blinkerSync:
		return "blinker-sync"
	case blinkerSyncing:
		return "blinker-syncing"
	case blinkerDownloading:
		return "blinker-downloading"
	case blinkerError:
		return "blinker-error"
	default:
		return ""
	}
}

// Blinker is the Blinker widget.
type Blinker struct {
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
	.blinker-sync,
	.blinker-syncing,
	.blinker-downloading,
	.blinker-error {
		transition: linear 100ms;
	}
	.blinker-sync {
		opacity: 0.8;
	}
	.blinker-syncing,
	.blinker-downloading {
		color:   alpha(@theme_fg_color, 0.5);
		opacity: 1;
	}
	.blinker-error {
		color:   red;
		opacity: 1;
	}
`)

// New creates a new blinker.
func New(ctx context.Context) *Blinker {
	img := gtk.NewImageFromIconName("content-loading-symbolic")
	img.SetIconSize(gtk.IconSizeNormal)
	blinkerCSS(img)

	b := &Blinker{Image: *img}

	client := gotktrix.FromContext(ctx)

	gtkutil.BindSubscribe(b, func() func() {
		return gtkutil.FuncBatcher(
			client.AddSyncInterceptFull(b.onRequest),
			client.OnSync(b.onSynced),
		)
	})

	return b
}

func (b *Blinker) onRequest(
	_ *http.Request,
	next func() (*http.Response, error)) (*http.Response, error) {

	glib.IdleAdd(b.syncing)

	r, err := next()
	if err != nil {
		glib.IdleAdd(func() { b.error(err) })
		return r, err
	}

	glib.IdleAdd(b.downloading)
	return r, nil
}

func (b *Blinker) onSynced(*api.SyncResponse) {
	glib.IdleAdd(b.sync)
}

func (b *Blinker) sync() {
	b.set(blinkerSync)
	b.prev = glib.TimeoutAdd(blinkerStayTime, func() {
		b.prev = 0
		b.cas(blinkerSync, blinkerNone)
	})
}

func (b *Blinker) syncing() {
	b.prev = glib.TimeoutSecondsAdd(2, func() {
		b.prev = 0
		b.cas(blinkerNone, blinkerSyncing)
	})
}

func (b *Blinker) downloading() {
	b.cas(blinkerSyncing, blinkerDownloading)
}

func (b *Blinker) error(err error) {
	b.set(blinkerError)
	b.SetTooltipText(err.Error())
}

func (b *Blinker) cas(ifThis, thenState blinkerState) bool {
	if b.state == ifThis {
		b.set(thenState)
		return true
	}
	return false
}

func (b *Blinker) set(state blinkerState) {
	b.SetTooltipText("")

	if b.state != blinkerNone {
		b.RemoveCSSClass(b.state.Class())
		b.state = blinkerNone
	}

	if b.prev != 0 {
		glib.SourceRemove(b.prev)
		b.prev = 0
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
