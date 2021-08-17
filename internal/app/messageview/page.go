package messageview

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/compose"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/components/autoscroll"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/pkg/errors"
)

// Page describes a tab page, which is a single message view. It satisfies teh
// MessageViewer interface.
type Page struct {
	gtk.Widgetter
	Composer *compose.Composer

	scroll   *autoscroll.Window
	list     *gtk.ListBox
	messages map[matrix.EventID]message.Message

	name    string
	onTitle func(title string)
	ctx     gtkutil.ContextTaker

	parent *View
	roomID matrix.RoomID

	loaded bool
}

var _ message.MessageViewer = (*Page)(nil)

var msgListCSS = cssutil.Applier("messageview-msglist", `
	.messageview-msglist {
		background: none;
	}
`)

var rhsCSS = cssutil.Applier("messageview-rhs", `
	.messageview-rhs > scrolledwindow > viewport {
		padding-bottom: 10px;
	}
	.messageview-rhs {
		background-image: linear-gradient(to top, @theme_base_color 0px, transparent 40px);
	}
`)

const (
	MessagesMaxWidth   = 1000
	MessagesClampWidth = 800
)

// NewPage creates a new page.
func NewPage(ctx context.Context, parent *View, roomID matrix.RoomID) *Page {
	name, _ := parent.client.Offline().RoomName(roomID)

	msgList := gtk.NewListBox()

	page := Page{
		list:     msgList,
		messages: make(map[matrix.EventID]message.Message),

		onTitle: func(string) {},
		ctx:     gtkutil.WithWidgetVisibility(ctx, msgList),
		name:    name,

		parent: parent,
		roomID: roomID,
	}

	page.list.SetSelectionMode(gtk.SelectionNone)
	msgListCSS(page.list)

	// Sort messages according to the timestamp.
	msgList.SetSortFunc(func(r1, r2 *gtk.ListBoxRow) int {
		t1 := page.messages[matrix.EventID(r1.Name())].Event().OriginServerTime()
		t2 := page.messages[matrix.EventID(r2.Name())].Event().OriginServerTime()

		if t1 < t2 {
			return -1
		}
		if t1 == t2 {
			return 0
		}
		return 1 // t1 > t2
	})

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(MessagesMaxWidth)
	clamp.SetTighteningThreshold(MessagesClampWidth)
	clamp.SetChild(page.list)

	page.scroll = autoscroll.NewWindow()
	page.scroll.SetVExpand(true)
	page.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	page.scroll.SetChild(clamp)

	// Bind the scrolled window for automatic scrolling.
	page.list.SetAdjustment(page.scroll.VAdjustment())

	page.Composer = compose.New(ctx, &page, roomID)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(page.scroll)
	box.Append(page.Composer)
	rhsCSS(box)

	// main widget
	page.Widgetter = box

	gtkutil.MapSubscriber(page, func() func() {
		return parent.client.SubscribeTimelineRaw(roomID, func(r *event.RawEvent) {
			glib.IdleAdd(func() { page.OnRoomEvent(r) })
		})
	})

	return &page
}

// OnTitle subscribes to the page's title changes.
func (p *Page) OnTitle(f func(string)) {
	p.onTitle = f

	if p.name == "" {
		f(string(p.roomID))
	} else {
		f(p.name)
	}
}

// LastMessage satisfies MessageViewer.
func (p *Page) LastMessage() message.Message {
	if len(p.messages) == 0 {
		return nil
	}

	lastRow := p.list.RowAtIndex(len(p.messages) - 1)

	if msg, ok := p.messages[matrix.EventID(lastRow.Name())]; ok {
		return msg
	}

	return nil
}

// RoomID returns this room's ID.
func (p *Page) RoomID() matrix.RoomID {
	return p.roomID
}

// RoomName returns this page's room name.
func (p *Page) RoomName() string {
	return p.name
}

// OnRoomEvent is called on every room event belonging to this room.
func (p *Page) OnRoomEvent(raw *event.RawEvent) {
	if raw.RoomID != p.roomID {
		return
	}

	p.onRoomEvent(raw)
	p.clean()
}

func (p *Page) clean() {
	if !p.scroll.IsBottomed() {
		return
	}

	for len(p.messages) >= gotktrix.TimelimeLimit {
		row := p.list.RowAtIndex(0)
		if row == nil {
			continue
		}

		p.list.Remove(row)
		delete(p.messages, matrix.EventID(row.Name()))
	}
}

func (p *Page) onRoomEvent(raw *event.RawEvent) {
	m := message.NewCozyMessage(p.parent.ctx, p, raw)

	p.messages[raw.ID] = m

	row := gtk.NewListBoxRow()
	row.SetName(string(raw.ID))
	row.SetChild(m)

	p.list.Append(row)
}

// Load asynchronously loads the page. The given callback is called once the
// page finishes loading.
func (p *Page) Load(done func()) {
	if p.loaded {
		done()
		return
	}

	ctx := p.ctx.Take()
	client := p.parent.client.WithContext(ctx)

	fetchName := p.name == ""

	go func() {
		if fetchName {
			// Update the name from state if possible.
			name, err := client.RoomName(p.roomID)
			if err == nil {
				glib.IdleAdd(func() {
					p.name = name
					p.onTitle(p.name)
				})
			}
		}

		events, err := client.RoomTimelineRaw(p.roomID)
		if err != nil {
			app.Error(ctx, errors.Wrap(err, "failed to load timeline"))
			glib.IdleAdd(done)
			return
		}

		glib.IdleAdd(func() {
			for i := range events {
				p.onRoomEvent(&events[i])
			}

			p.loaded = true
			p.scroll.ScrollToBottom()
			done()
		})
	}()
}
