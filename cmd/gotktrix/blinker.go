package main

import (
	"context"

	"github.com/chanbakjsd/gotrix/api"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
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
	blinkerSyncIcon  = "emblem-favorite-symbolic"
	blinkerErrorIcon = "dialog-error-symbolic"
	blinkerStayTime  = 200 // ms
)

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
	.blinker {
		color:   @accent_color;
		margin:  6px;
		opacity: 0;
		transition: linear 650ms;
		transition-property: opacity, color;
	}
	.blinker-sync {
		opacity: 1;
		transition: none;
	}
	.blinker-error {
		color: red;
	}
`)

func Blinker(ctx context.Context) gtk.Widgetter {
	img := gtk.NewImageFromIconName(blinkerSyncIcon)
	img.SetIconSize(gtk.IconSizeNormal)
	img.SetOpacity(0.8)
	blinkerCSS(img)

	b := &blinker{Image: *img}
	cancel := gotktrix.FromContext(ctx).OnSync(func(*api.SyncResponse) {
		b.sync()
	})

	win := app.Window(ctx)
	win.Connect("destroy", cancel)

	return b
}

func (b *blinker) sync() {
	b.SetFromIconName(blinkerSyncIcon)
	b.set(blinkerSync)

	if b.prev != 0 {
		glib.SourceRemove(b.prev)
	}

	b.prev = glib.TimeoutAdd(blinkerStayTime, func() {
		b.set(blinkerNone)
		b.prev = 0
	})
}

func (b *blinker) error() {
	b.SetFromIconName(blinkerErrorIcon)
	b.set(blinkerError)
}

func (b *blinker) set(state blinkerState) {
	if b.state != blinkerNone {
		b.RemoveCSSClass(b.state.Class())
		b.state = blinkerNone
	}

	b.state = state
	if state != blinkerNone {
		b.AddCSSClass(state.Class())
	}
}

func (b *blinker) cas(state, ifState blinkerState) {
	if b.state != ifState {
		return
	}

	b.RemoveCSSClass(ifState.Class())
	b.state = blinkerNone

	b.set(state)
}
