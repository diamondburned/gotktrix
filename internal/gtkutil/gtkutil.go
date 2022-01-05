package gtkutil

import (
	"context"
	"sync"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
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
	drag.SetContent(gdk.NewContentProviderForValue(coreglib.NewValue(v)))

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
	gval := coreglib.NewValue(dst)

	drag := NewDragSourceWithContent(w, a, gval)

	drop := gtk.NewDropTarget(gval.Type(), a)
	drop.Connect("drop", func(drop *gtk.DropTarget, src *coreglib.Value, x, y float64) {
		log.Println("dropped at", y, "from", dst, "to", src.GoValue())
	})

	w.AddController(drag)
	w.AddController(drop)
}
*/

// NewListDropTarget creates a new DropTarget that highlights the row.
func NewListDropTarget(l *gtk.ListBox, typ coreglib.Type, actions gdk.DragAction) *gtk.DropTarget {
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

// OnFirstMap attaches f to be called on the first time the widget is mapped on
// the screen.
func OnFirstMap(w gtk.Widgetter, f func()) {
	widget := gtk.BaseWidget(w)
	if widget.Mapped() {
		f()
		return
	}

	var handle glib.SignalHandle
	handle = widget.ConnectMap(func() {
		f()
		widget.HandlerDisconnect(handle)
	})
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

// OnFirstDrawUntil attaches f to be called on the first time the widget is
// drawn on the screen. f is called again until it returns false.
func OnFirstDrawUntil(w gtk.Widgetter, f func() bool) {
	widget := gtk.BaseWidget(w)
	widget.AddTickCallback(func(_ gtk.Widgetter, clocker gdk.FrameClocker) bool {
		return gdk.BaseFrameClock(clocker).FPS() == 0 || f()
	})
}

// SignalToggler is a small helper to allow binding the same signal to different
// objects while unbinding the previous one.
func SignalToggler(signal string, f interface{}) func(obj coreglib.Objector) {
	var lastObj coreglib.Objector
	var lastSig coreglib.SignalHandle

	return func(obj coreglib.Objector) {
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

// BindSubscribe calls f when w gets mapped.
func BindSubscribe(widget gtk.Widgetter, f func() (unsub func())) {
	w := gtk.BaseWidget(widget)
	var unsub func()
	w.ConnectMap(func() {
		unsub = f()
	})
	w.ConnectUnmap(func() {
		unsub()
	})
}

var mainThread = glib.MainContextDefault()

// InvokeMain invokes f in the main loop. It is useful in global helper
// functions where it's unclear where the caller will invoke it from, but it
// should be used carefully, since it's easy to be abused.
func InvokeMain(f func()) {
	if mainThread.IsOwner() {
		// fast path
		f()
		return
	}

	// I'm going to abuse the shit out of this.
	done := make(chan struct{}, 1)
	mainThread.InvokeFull(int(coreglib.PriorityHigh), func() bool {
		f()
		done <- struct{}{}
		return false
	})
	<-done
}

// InMain returns true if the current execution thread is the main thread. It is
// useful for guarding thread-unsafe init functions.
// func InMain() bool {
// 	return mainThread.IsOwner()
// }

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

		coreglib.IdleAdd(func() {
			select {
			case <-ctx.Done():
			default:
				fn()
			}
		})
	}()
}

var (
	scaleFactor      int
	scaleFactorMutex sync.RWMutex
	initScaleOnce    sync.Once
)

// ScaleFactor returns the largest scale factor from all the displays. It is
// thread-safe.
func ScaleFactor() int {
	initScale()

	scaleFactorMutex.RLock()
	defer scaleFactorMutex.RUnlock()

	if scaleFactor == 0 {
		panic("uninitialized scaleFactor")
	}

	return scaleFactor
}

// SetScaleFactor sets the global maximum scale factor. This function is useful
// of GDK fails to update the scale factor in time.
func SetScaleFactor(maxScale int) {
	scaleFactorMutex.Lock()
	defer scaleFactorMutex.Unlock()

	if scaleFactor < maxScale {
		scaleFactor = maxScale
	}
}

func initScale() {
	initScaleOnce.Do(func() {
		InvokeMain(func() {
			dmanager := gdk.DisplayManagerGet()
			dmanager.Connect("display-opened", func(display *gdk.Display) {
				bindDisplay(display.Monitors())
			})
			for _, display := range dmanager.ListDisplays() {
				bindDisplay(display.Monitors())
			}
		})
	})
}

func bindDisplay(monitors gio.ListModeller) {
	monitors.Connect("items-changed", func() { updateScale(monitors) })
	updateScale(monitors)
}

func updateScale(monitors gio.ListModeller) {
	maxScale := 0

	eachMonitor(monitors, func(monitor *gdk.Monitor) {
		if scale := monitor.ScaleFactor(); maxScale < scale {
			maxScale = scale
		}
	})

	scaleFactorMutex.Lock()
	defer scaleFactorMutex.Unlock()

	if scaleFactor < maxScale {
		scaleFactor = maxScale
	}
}

func eachMonitor(list gio.ListModeller, f func(*gdk.Monitor)) {
	var i uint
	obj := list.Item(0)

	for obj != nil {
		f(obj.Cast().(*gdk.Monitor))

		i++
		obj = list.Item(i)
	}
}
