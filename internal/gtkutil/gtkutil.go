package gtkutil

import (
	"context"
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

var _ = cssutil.WriteCSS(`
	.dragging {
		background-color: @theme_bg_color;
	}
`)

// NewDragSourceWithContent creates a new DragSource with the given Go value.
func NewDragSourceWithContent(w gtk.Widgetter, a gdk.DragAction, v interface{}) *gtk.DragSource {
	drag := gtk.NewDragSource()
	drag.SetActions(a)
	drag.SetContent(gdk.NewContentProviderForValue(glib.NewValue(v)))

	widget := gtk.BaseWidget(w)

	paint := gtk.NewWidgetPaintable(w)
	drag.Connect("drag-begin", func() {
		widget.AddCSSClass("dragging")
		drag.SetIcon(paint, 0, 0)
	})
	drag.Connect("drag-end", func() {
		widget.RemoveCSSClass("dragging")
	})

	return drag
}

/*
// DragDroppable describes a widget that can be dragged and dropped.
type DragDroppable interface {
	gtk.Widgetter
	// DragData returns the data of this drag-droppable instance.
	DragData() (interface{}, gdk.DragAction)
	// OnDropped is called when another widget is dropped onto.
	OnDropped(src interface{}, pos gtk.PositionType)
}

// BindDragDrop binds the current widget as a simultaneous drag source and drop
// target.
func BindDragDrop(w gtk.Widgetter, a gdk.DragAction, dst interface{}, f func(gtk.PositionType)) {
	gval := glib.NewValue(dst)

	drag := NewDragSourceWithContent(w, a, gval)

	drop := gtk.NewDropTarget(gval.Type(), a)
	drop.Connect("drop", func(drop *gtk.DropTarget, src *glib.Value, x, y float64) {
		log.Println("dropped at", y, "from", dst, "to", src.GoValue())
	})

	w.AddController(drag)
	w.AddController(drop)
}
*/

// NewListDropTarget creates a new DropTarget that highlights the row.
func NewListDropTarget(l *gtk.ListBox, typ glib.Type, actions gdk.DragAction) *gtk.DropTarget {
	drop := gtk.NewDropTarget(typ, actions)
	drop.Connect("motion", func(drop *gtk.DropTarget, x, y float64) gdk.DragAction {
		if row := l.RowAtY(int(y)); row != nil {
			l.DragHighlightRow(row)
			return actions
		}
		return 0
	})
	drop.Connect("leave", func() {
		l.DragUnhighlightRow()
	})
	return drop
}

// RowAtY returns the row as well as the position type (top or bottom) relative
// to that row.
func RowAtY(list *gtk.ListBox, y float64) (*gtk.ListBoxRow, gtk.PositionType) {
	row := list.RowAtY(int(y))
	if row == nil {
		return nil, 0
	}

	r, ok := row.ComputeBounds(list)
	if ok {
		// Calculate the Y position from the widget's top.
		pos := y - float64(r.Y())
		// Divide the height by 2 and check the bounds.
		mid := float64(r.Height()) / 2

		if pos > mid {
			// More than half, so bottom.
			return row, gtk.PosBottom
		} else {
			return row, gtk.PosTop
		}
	}

	// Default to bottom.
	return row, gtk.PosBottom
}

// OnFirstDraw attaches f to be called on the first time the widget is drawn on
// the screen.
func OnFirstDraw(w gtk.Widgetter, f func()) {
	widget := gtk.BaseWidget(w)
	widget.AddTickCallback(func(_ gtk.Widgetter, clocker gdk.FrameClocker) bool {
		if clock := gdk.BaseFrameClock(clocker); clock.FPS() > 0 {
			f()
			return false
		}
		return true // retry
	})
}

// SignalToggler is a small helper to allow binding the same signal to different
// objects while unbinding the previous one.
func SignalToggler(signal string, f interface{}) func(obj glib.Objector) {
	var lastObj glib.Objector
	var lastSig glib.SignalHandle

	return func(obj glib.Objector) {
		if lastObj != nil && lastSig != 0 {
			lastObj.HandlerDisconnect(lastSig)
		}

		if obj == nil {
			lastObj = nil
			lastSig = 0
			return
		}

		lastObj = obj
		lastSig = obj.Connect(signal, f)
	}
}

// Async runs asyncFn in a goroutine and runs the returned callback in the main
// thread. If ctx is cancelled during, the returned callback will not be called.
func Async(ctx context.Context, asyncFn func() func()) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	go func() {
		fn := asyncFn()
		if fn == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		glib.IdleAdd(func() {
			select {
			case <-ctx.Done():
			default:
				fn()
			}
		})
	}()
}

var (
	scaleFactor      int = -1
	scaleFactorMutex sync.RWMutex
	initScaleOnce    sync.Once
)

// ScaleFactor returns the largest scale factor from all the displays. It is
// thread-safe.
func ScaleFactor() int {
	initScale()

	scaleFactorMutex.RLock()
	defer scaleFactorMutex.RUnlock()

	if scaleFactor == -1 {
		panic("uninitialized scaleFactor")
	}

	return scaleFactor
}

func initScale() {
	initScaleOnce.Do(func() {
		dmanager := gdk.DisplayManagerGet()
		dmanager.Connect("display-opened", func(dmanager *gdk.DisplayManager) {
			updateScale(dmanager)
		})
		updateScale(dmanager)
	})
}

func updateScale(dmanager *gdk.DisplayManager) {
	maxScale := 1

	for _, display := range dmanager.ListDisplays() {
		if display.IsClosed() {
			continue
		}

		monitors := display.Monitors()
		for i, len := uint(0), monitors.NItems(); i < len; i++ {
			monitor := monitors.Item(i).Cast().(*gdk.Monitor)

			if scale := monitor.ScaleFactor(); maxScale < scale {
				maxScale = scale
			}
		}
	}

	scaleFactorMutex.Lock()
	defer scaleFactorMutex.Unlock()

	if scaleFactor < maxScale {
		scaleFactor = maxScale
	}
}
