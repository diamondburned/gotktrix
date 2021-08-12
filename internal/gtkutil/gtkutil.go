package gtkutil

import (
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

	paint := gtk.NewWidgetPaintable(w)
	drag.Connect("drag-begin", func() {
		w.AddCSSClass("dragging")
		drag.SetIcon(paint, 0, 0)
	})
	drag.Connect("drag-end", func() {
		w.RemoveCSSClass("dragging")
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

// MapSubscriber maps any subscriber callback that can unsubscribe to a widget's
// map and unmap signals.
func MapSubscriber(w gtk.Widgetter, sub func() (unsub func())) {
	var unsub func()
	w.Connect("map", func() {
		unsub = sub()
	})
	w.Connect("unmap", func() {
		unsub()
	})
}
