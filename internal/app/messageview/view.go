package messageview

import (
	"log"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
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
func (p *pages) SetVisible(visible matrix.RoomID) {
	p.visible = visible

	if _, ok := p.pages[visible]; !ok {
		log.Panicf("selected room %s not in page registry", visible)
	}
}

func (p *pages) PopVisible() *tabPage {
	return p.Pop(p.visible)
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

	app    Application
	client *gotktrix.Client

	pages *pages
}

type Application interface {
	app.Applicationer
	SetSelectedRoom(id matrix.RoomID)
}

// New creates a new instance of View.
func New(app Application) *View {
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

	view.Connect("notify::selected-page", func(view *adw.TabView) {
		child := view.SelectedPage().Child()
		pages.SetVisible(matrix.RoomID(child.Name()))
	})

	view.Connect("close-page", func(view *adw.TabView, page *adw.TabPage) {
		// Delete the page from the page registry.
		child := page.Child()
		pages.Pop(matrix.RoomID(child.Name()))
		// Finish closing the page.
		view.ClosePageFinish(page, true)

		if view.NPages() == 0 {
			// Restore the stack to the placeholder if available.
			if empty := stack.ChildByName("empty"); empty != nil {
				stack.SetVisibleChild(empty)
			}
		}
	})

	return &View{
		Stack: stack,
		box:   box,
		view:  view,
		bar:   bar,

		app:    app,
		client: app.Client(),

		pages: pages,
	}
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
	// Break up a potential infinite call recursion.
	if v.pages.visible == id {
		return v.pages.pages[id].Page
	}

	v.Stack.SetVisibleChild(v.box)

	if page, ok := v.pages.pages[id]; ok {
		v.view.SetSelectedPage(page.tab)
		return page.Page
	}

	page := &tabPage{Page: NewPage(v, id)}
	page.SetName(string(id))

	v.pages.pages[id] = page

	if v.pages.visible != "" && !newTab {
		last := v.pages.PopVisible()

		page.tab = v.view.AddPage(page.Page, last.tab)
		v.view.SetSelectedPage(page.tab)
		v.view.ClosePage(last.tab)
	} else {
		page.tab = v.view.Append(page)
		v.view.SetSelectedPage(page.tab)
	}

	page.OnTitle(page.tab.SetTitle)

	page.tab.SetLoading(true)
	page.Load(func() { page.tab.SetLoading(false) })

	return page.Page
}
