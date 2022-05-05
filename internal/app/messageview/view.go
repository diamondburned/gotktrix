package messageview

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/matrix"
)

// View describes a view for multiple message views.
type View struct {
	*gtk.Stack
	empty gtk.Widgetter

	ctx    context.Context
	ctrl   Controller
	client *gotktrix.Client

	current *Page
}

// This is used for tabs, but we're not implementing tabs for now.

type Controller interface {
	SetSelectedRoom(id matrix.RoomID)
}

// New creates a new instance of View.
func New(ctx context.Context, ctrl Controller) *View {
	// view := gtk.NewNotebook()
	// view.SetVExpand(true)
	// view.ConnectAfter("page-removed", func() {
	// 	view.SetShowTabs(view.NPages() > 0)
	// })

	stack := gtk.NewStack()
	stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)

	return &View{
		Stack:  stack,
		ctx:    ctx,
		ctrl:   ctrl,
		client: gotktrix.FromContext(ctx),
	}
}

// SetPlaceholder sets the placeholder widget.
func (v *View) SetPlaceholder(w gtk.Widgetter) {
	v.Stack.AddChild(w)

	if v.empty != nil {
		v.Stack.Remove(v.empty)
	}
	v.empty = w

	if v.current == nil {
		v.Stack.SetVisibleChild(w)
	}
}

// OpenRoom opens a Matrix room on the curernt page. If there is no page yet,
// then a new one is created. If the room already exists in another tab, then
// that tab is selected.
func (v *View) OpenRoom(id matrix.RoomID) *Page {
	return v.openRoom(id, false)
}

/*
// OpenRoomInNewTab opens the room in a new tab. If the room is already opened,
// then the old tab is focused. If no rooms are opened yet, then the first tab
// is created, so the function behaves like OpenRoom.
func (v *View) OpenRoomInNewTab(id matrix.RoomID) *Page {
	return v.openRoom(id, true)
}
*/

func (v *View) openRoom(id matrix.RoomID, newTab bool) *Page {
	// Break up a potential infinite call recursion.
	if v.current != nil && v.current.roomID == id {
		return v.current
	}

	page := NewPage(v.ctx, v, id)
	page.Load()

	gtk.BaseWidget(page).SetName(string(id))

	v.Stack.AddChild(page)
	v.Stack.SetVisibleChild(page)

	if v.current != nil {
		v.Stack.Remove(v.current)
	}
	v.current = page

	return page
}

// Current returns the current page or nil if none.
func (v *View) Current() *Page {
	return v.current
}
