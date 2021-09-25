package messageview

import (
	"context"
	"fmt"
	"log"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
)

type pages struct {
	pages   map[matrix.RoomID]*tabPage
	visible matrix.RoomID
}

func newPages() *pages {
	return &pages{
		pages:   make(map[matrix.RoomID]*tabPage, 1),
		visible: "",
	}
}

type tabPage struct {
	*Page
	tab *adw.TabPage
}

// SetVisible sets the active room ID and verify that it's present.
func (p *pages) SetVisible(visible matrix.RoomID) *Page {
	p.visible = visible

	if visible == "" {
		return nil
	}

	pg, ok := p.pages[visible]
	if !ok {
		log.Panicf("selected room %s not in page registry", visible)
	}

	return pg.Page
}

func (p *pages) FromTab(tab *adw.TabPage) *tabPage {
	child := tab.Child()
	return p.pages[matrix.RoomID(child.Name())]
}

func (p *pages) Visible() *tabPage {
	return p.pages[p.visible]
}

func (p *pages) Pop(id matrix.RoomID) *tabPage {
	page := p.pages[id]
	delete(p.pages, id)
	return page
}

// View describes a view for multiple message views.
type View struct {
	*gtk.Stack
	box *gtk.Box

	view  *adw.TabView
	bar   *adw.TabBar
	empty gtk.Widgetter

	ctx    context.Context
	ctrl   Controller
	client *gotktrix.Client

	pages *pages
}

type Controller interface {
	SetSelectedRoom(id matrix.RoomID)
}

// New creates a new instance of View.
func New(ctx context.Context, ctrl Controller) *View {
	view := adw.NewTabView()
	view.SetVExpand(true)

	bar := adw.NewTabBar()
	bar.SetAutohide(true)
	bar.SetView(view)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(bar)
	box.Append(view)

	stack := gtk.NewStack()
	stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	stack.AddNamed(box, "named")

	pages := newPages()

	setTitle := func(tab *adw.TabPage) {
		if tab != nil {
			app.SetTitle(ctx, fmt.Sprintf("%s — gotktrix", tab.Title()))
		} else {
			app.SetTitle(ctx, "gotktrix")
		}
	}

	// Keep track of the last signal handler with the last page that's used to
	// update the window title.
	pageToggler := gtkutil.SignalToggler("notify::title", setTitle)

	view.Connect("notify::selected-page", func() {
		stack.SetVisibleChild(box)

		selected := view.SelectedPage()

		if selected == nil {
			setTitle(nil)
			pageToggler(nil)

			ctrl.SetSelectedRoom("")
			pages.SetVisible("")
			return
		}

		setTitle(selected)
		pageToggler(selected)

		child := selected.Child()
		rpage := pages.SetVisible(matrix.RoomID(child.Name()))
		rpage.MarkAsRead()

		ctrl.SetSelectedRoom(rpage.roomID)
	})

	view.Connect("close-page", func(view *adw.TabView, page *adw.TabPage) {
		// Delete the page from the page registry.
		child := page.Child()
		pages.Pop(matrix.RoomID(child.Name()))
	})

	return &View{
		Stack: stack,
		box:   box,
		view:  view,
		bar:   bar,

		ctx:    ctx,
		ctrl:   ctrl,
		client: gotktrix.FromContext(ctx),

		pages: pages,
	}
}

func updateWindowTitle(ctx context.Context, page *Page) {
}

// SetPlaceholder sets the placeholder widget.
func (v *View) SetPlaceholder(w gtk.Widgetter) {
	if v.empty != nil {
		v.Stack.Remove(v.empty)
	}

	v.Stack.AddNamed(w, "empty")
	v.empty = w

	if v.view.NPages() == 0 {
		v.Stack.SetVisibleChild(w)
	}
}

// OpenRoom opens a Matrix room on the curernt page. If there is no page yet,
// then a new one is created. If the room already exists in another tab, then
// that tab is selected.
func (v *View) OpenRoom(id matrix.RoomID) *Page {
	return v.openRoom(id, false)
}

// OpenRoomInNewTab opens the room in a new tab. If the room is already opened,
// then the old tab is focused. If no rooms are opened yet, then the first tab
// is created, so the function behaves like OpenRoom.
func (v *View) OpenRoomInNewTab(id matrix.RoomID) *Page {
	return v.openRoom(id, true)
}

func (v *View) openRoom(id matrix.RoomID, newTab bool) *Page {
	visible := v.pages.Visible()

	// Break up a potential infinite call recursion.
	if visible != nil && visible.roomID == id {
		return visible.Page
	}

	page, ok := v.pages.pages[id]
	if !ok {
		page = &tabPage{Page: NewPage(v.ctx, v, id)}
		page.SetName(string(id))

		v.pages.pages[id] = page

		// Why does Append trigger a selected-page signal? I have no idea. But
		// it does, so we have to add the page into the registry before this.
		page.tab = v.view.Append(page)

		page.OnTitle(func(title string) {
			page.tab.SetTitle(title)

			if v.pages.visible == page.roomID {
				updateWindowTitle(v.ctx, page.Page)
			}
		})

		page.tab.SetLoading(true)
		page.Load(func() { page.tab.SetLoading(false) })

		// Close the previous tab if we're not opening in a new tab.
		if visible != nil && !newTab {
			v.view.ClosePage(visible.tab)
		}
	}

	v.view.SetSelectedPage(page.tab)

	return page.Page
}
