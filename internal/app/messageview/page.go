package messageview

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/components/autoscroll"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/pkg/errors"
)

// Page describes a tab page, which is a single message view. It satisfies teh
// MessageViewer interface.
type Page struct {
	*autoscroll.Window
	list     *gtk.ListBox
	name     string
	messages map[matrix.EventID]message.Message

	onTitle func(title string)
	cancel  gtkutil.Canceler

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

// NewPage creates a new page.
func NewPage(parent *View, roomID matrix.RoomID) *Page {
	msgList := gtk.NewListBox()
	msgList.SetSelectionMode(gtk.SelectionNone)
	msgListCSS(msgList)

	name, _ := parent.client.Offline().RoomName(roomID)

	msgMap := map[matrix.EventID]message.Message{}

	// Sort messages according to the timestamp.
	msgList.SetSortFunc(func(r1, r2 *gtk.ListBoxRow) int {
		t1 := msgMap[matrix.EventID(r1.Name())].Event().OriginServerTime()
		t2 := msgMap[matrix.EventID(r2.Name())].Event().OriginServerTime()

		if t1 < t2 {
			return -1
		}
		if t1 == t2 {
			return 0
		}
		return 1 // t1 > t2
	})

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(1000)
	clamp.SetTighteningThreshold(800)
	clamp.SetChild(msgList)

	scroll := autoscroll.NewWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(clamp)

	// Bind the scrolled window for automatic scrolling.
	msgList.SetAdjustment(scroll.VAdjustment())

	return &Page{
		Window: scroll,

		list:     msgList,
		name:     name,
		messages: msgMap,

		onTitle: func(string) {},
		cancel:  gtkutil.NewCanceler(),

		parent: parent,
		roomID: roomID,
	}
}

// Stop stops any asynchronous work inside the page.
func (p *Page) Stop() {
	p.cancel.Cancel()
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

// Client satisfies MessageViewer.
func (p *Page) Client() *gotktrix.Client {
	return p.parent.client
}

// Context returns the page's context
func (p *Page) Context() context.Context {
	return p.cancel.Context()
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

// OnRoomEvent is called on every room event belonging to this room.
func (p *Page) OnRoomEvent(ev event.RoomEvent) {
	if ev.Room() != p.roomID {
		return
	}

	p.onRoomEvent(ev)
}

func (p *Page) onRoomEvent(ev event.RoomEvent) {
	m := message.NewCozyMessage(p, ev)

	eventID := m.Event().ID()
	p.messages[eventID] = m

	row := gtk.NewListBoxRow()
	row.SetName(string(eventID))
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

	ctx := p.cancel.Context()
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

		events, err := client.RoomTimeline(p.roomID)
		if err != nil {
			p.parent.app.Error(errors.Wrap(err, "failed to load timeline"))
			glib.IdleAdd(done)
			return
		}

		glib.IdleAdd(func() {
			for _, ev := range events {
				p.onRoomEvent(ev)
			}

			p.loaded = true
			p.ScrollToBottom()
			done()
		})
	}()
}
