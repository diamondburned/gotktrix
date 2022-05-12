package blinker

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/api"
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
	ctx context.Context

	prev  glib.SourceHandle
	state blinkerState
	last  time.Time

	rmut    sync.Mutex
	rctx    context.Context
	rcancel context.CancelFunc
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

	b := &Blinker{
		Image: *img,
		ctx:   ctx,
	}

	client := gotktrix.FromContext(ctx)
	b.rctx, b.rcancel = context.WithCancel(ctx)

	gtkutil.BindSubscribe(b, func() func() {
		return gtkutil.FuncBatcher(
			client.AddSyncInterceptFull(b.onRequest),
			client.OnSync(b.onSynced),
		)
	})

	gtkutil.BindActionMap(b, map[string]func(){
		"blinker.stop-request": func() {
			b.rmut.Lock()
			defer b.rmut.Unlock()

			b.rcancel()
			b.rctx, b.rcancel = context.WithCancel(ctx)
		},
	})

	gtkutil.BindPopoverMenu(b, gtk.PosBottom, [][2]string{
		{"_Break", "blinker.stop-request"},
	})

	return b
}

func (b *Blinker) onRequest(
	req *http.Request,
	next func() (*http.Response, error)) (*http.Response, error) {

	glib.IdleAdd(b.syncing)

	b.rmut.Lock()
	ctx := b.rctx
	b.rmut.Unlock()

	*req = *req.WithContext(ctx)

	r, err := next()
	if err != nil {
		glib.IdleAdd(func() { b.error(err) })
		return r, err
	}

	glib.IdleAdd(b.downloading)
	return r, nil
}

func (b *Blinker) onSynced(*api.SyncResponse) {
	now := time.Now()
	glib.IdleAdd(func() { b.sync(now) })
}

func (b *Blinker) sync(now time.Time) {
	b.set(blinkerSync)
	b.last = now
	b.prev = glib.TimeoutAddPriority(blinkerStayTime, glib.PriorityDefaultIdle, func() {
		b.prev = 0
		b.cas(blinkerSync, blinkerNone)
	})
}

func (b *Blinker) syncing() {
	b.prev = glib.TimeoutSecondsAddPriority(2, glib.PriorityDefaultIdle, func() {
		b.prev = 0
		b.cas(blinkerNone, blinkerSyncing)
	})
}

func (b *Blinker) downloading() {
	b.cas(blinkerSyncing, blinkerDownloading)
}

func (b *Blinker) error(err error) {
	b.set(blinkerError)
	b.SetTooltipMarkup(locale.FromContext(b.ctx).Sprintf(
		`<span color="red"><b>Error:</b></span> %s`,
		err.Error(),
	))
}

func (b *Blinker) cas(ifThis, thenState blinkerState) bool {
	if b.state == ifThis {
		b.set(thenState)
		return true
	}
	return false
}

func (b *Blinker) set(state blinkerState) {
	b.SetTooltipText(b.tooltipText())

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

func (b *Blinker) tooltipText() string {
	if !b.last.IsZero() {
		return locale.Sprintf(b.ctx, "Last synced %s", locale.Time(b.last, true))
	}
	return ""
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
