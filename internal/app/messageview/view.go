package messageview

import (
	"context"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
)

// View describes a view for multiple message views.
type View struct {
	*gtk.Stack
	view  *adaptive.Bin
	empty gtk.Widgetter

	ctx    context.Context
	ctrl   Controller
	client *gotktrix.Client

	current *Page
}

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

	view := adaptive.NewBin()

	stack := gtk.NewStack()
	stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	stack.AddNamed(view, "named")

	return &View{
		Stack:  stack,
		view:   view,
		ctx:    ctx,
		ctrl:   ctrl,
		client: gotktrix.FromContext(ctx),
	}
}

func updateWindowTitle(ctx context.Context, notebook *gtk.Notebook, page gtk.Widgetter) {
	if page != nil {
		label := notebook.TabLabel(page).(*gtk.Label)
		app.SetTitle(ctx, label.Text())
	} else {
		app.SetTitle(ctx, "")
	}
}

// SetPlaceholder sets the placeholder widget.
func (v *View) SetPlaceholder(w gtk.Widgetter) {
	if v.empty != nil {
		v.Stack.Remove(v.empty)
	}

	v.Stack.AddNamed(w, "empty")
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
	v.Stack.SetVisibleChild(v.view)

	// Break up a potential infinite call recursion.
	if v.current != nil && v.current.roomID == id {
		return v.current
	}

	page := NewPage(v.ctx, v, id)
	page.Load(func() {})
	gtk.BaseWidget(page).SetName(string(id))

	v.current = page
	v.view.SetChild(page)

	page.OnTitle(func(title string) {
		app.SetTitle(v.ctx, page.name)
	})
	return page
}
