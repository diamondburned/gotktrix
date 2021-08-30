package messageview

import (
	"context"
	"log"

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
	messages map[matrix.EventID]messageRow

	name    string
	onTitle func(title string)
	ctx     gtkutil.ContextTaker

	parent *View
	roomID matrix.RoomID

	editing    matrix.EventID
	replyingTo matrix.EventID

	loaded bool
}

type messageRow struct {
	msg message.Message
	row *gtk.ListBoxRow
}

var _ message.MessageViewer = (*Page)(nil)

var msgListCSS = cssutil.Applier("messageview-msglist", `
	.messageview-msglist {
		background: none;
	}
	.messageview-msglist > * {
		border-right: 2px solid transparent;
	}
	.messageview-replyingto {
		border-right: 2px solid @accent_fg_color;
		background-color: alpha(@accent_bg_color, 0.35);
		background-image: -gtk-icontheme("mail-reply-sender-symbolic");
		background-size: 18px;
		background-repeat: no-repeat;
		background-position: calc(100% - 5px) 5px;
	}
	.messageview-replyingto:hover {
		background-color: alpha(@accent_bg_color, 0.45);
	}
`)

var rhsCSS = cssutil.Applier("messageview-rhs", `
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
		messages: make(map[matrix.EventID]messageRow),

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
		t1 := page.messages[matrix.EventID(r1.Name())].msg.Event().OriginServerTime()
		t2 := page.messages[matrix.EventID(r2.Name())].msg.Event().OriginServerTime()

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

	vp := gtk.NewViewport(nil, nil)
	vp.SetVScrollPolicy(gtk.ScrollNatural)
	vp.SetChild(clamp)

	page.scroll = autoscroll.NewWindow()
	page.scroll.SetVExpand(true)
	page.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	page.scroll.SetChild(vp)

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
		return parent.client.SubscribeTimeline(roomID, func(r *event.RawEvent) {
			glib.IdleAdd(func() { page.OnRoomEvent(r) })
		})
	})

	page.scroll.OnBottomed(func(bottomed bool) {
		if bottomed {
			// Mark the latest message as read everytime the user scrolls down
			// to the bottom.
			page.MarkAsRead()
		}
	})

	return &page
}

// IsActive returns true if this page is the one the user is viewing.
func (p *Page) IsActive() bool {
	return p.parent.pages.visible == p.roomID
}

// OnTitle subscribes to the page's title changes.
func (p *Page) OnTitle(f func(string)) {
	p.onTitle = f
	f(p.RoomName())
}

// LastMessage satisfies MessageViewer.
func (p *Page) LastMessage() message.Message {
	if len(p.messages) == 0 {
		return nil
	}

	lastRow := p.list.RowAtIndex(len(p.messages) - 1)

	if row, ok := p.messages[matrix.EventID(lastRow.Name())]; ok {
		return row.msg
	}

	return nil
}

// RoomID returns this room's ID.
func (p *Page) RoomID() matrix.RoomID {
	return p.roomID
}

// RoomName returns this page's room name.
func (p *Page) RoomName() string {
	if p.name == "" {
		return string(p.roomID)
	}
	return p.name
}

// OnRoomEvent is called on every room event belonging to this room.
func (p *Page) OnRoomEvent(raw *event.RawEvent) {
	if raw.RoomID != p.roomID {
		return
	}

	p.onRoomEvent(raw)
	p.clean()
	p.MarkAsRead()
}

// MarkAsRead marks the room as read.
func (p *Page) MarkAsRead() {
	if !p.IsActive() || !p.scroll.IsBottomed() || len(p.messages) == 0 {
		return
	}

	row := p.list.RowAtIndex(len(p.messages) - 1)
	if row == nil {
		// No row found despite p.messages having something. This is a bug.
		return
	}

	client := gotktrix.FromContext(p.ctx.Take())
	latest := matrix.EventID(row.Name())

	go func() {
		if err := client.MarkRoomAsRead(p.roomID, latest); err != nil {
			// No need to interrupt the user for this.
			log.Println("failed to mark room as read:", err)
		}
	}()
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

	row := gtk.NewListBoxRow()
	row.SetName(string(raw.ID))
	row.SetChild(m)

	p.messages[raw.ID] = messageRow{
		msg: m,
		row: row,
	}

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

func (p *Page) Edit(eventID matrix.EventID) {
	if p.replyingTo != "" {
		// Stop replying.
		p.ReplyTo("")
	}
}

// ReplyTo sets the event ID that the user wants to reply to.
func (p *Page) ReplyTo(eventID matrix.EventID) {
	if p.editing != "" {
		// Stop editing.
		p.Edit("")
	}

	const class = "messageview-replyingto"

	if p.replyingTo != "" {
		r, ok := p.messages[p.replyingTo]
		if ok {
			r.row.RemoveCSSClass(class)
		}
		p.replyingTo = ""
	}

	mr, ok := p.messages[eventID]
	if !ok {
		p.Composer.ReplyTo("")
		return
	}

	p.Composer.ReplyTo(eventID)
	mr.row.AddCSSClass(class)

	p.replyingTo = eventID
}
