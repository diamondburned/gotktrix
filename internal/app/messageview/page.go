package messageview

import (
	"context"
	"encoding/json"
	"log"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/compose"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/components/autoscroll"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
	"github.com/pkg/errors"
)

// Page describes a tab page, which is a single message view. It satisfies teh
// MessageViewer interface.
type Page struct {
	gtk.Widgetter
	Composer *compose.Composer

	extra *extraRevealer

	scroll *autoscroll.Window
	list   *gtk.ListBox
	// TODO: it might be better to refactor these maps into a map of only an
	// event object that simultaneously has a linked anchor. This way, there's
	// no need to keep two separate maps, and there's no need to handle small
	// pieces of events in separate places.
	messages map[matrix.EventID]messageRow
	mrelated map[matrix.EventID]matrix.EventID // keep track of reactions

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
	msg  message.Message
	row  *gtk.ListBoxRow
	sent matrix.Timestamp
}

var _ message.MessageViewer = (*Page)(nil)

var msgListCSS = cssutil.Applier("messageview-msglist", `
	.messageview-msglist {
		background: none;
		margin-bottom: .8em; /* for the extraRevealer */
	}
	.messageview-msglist > row {
		border-right: 2px solid transparent;
		transition: linear 150ms background-color;
		background: none;
		background-image: none;
		background-color: transparent;
	}
	.messageview-msglist > row:hover {
		background-color: alpha(@theme_fg_color, 0.1);
		transition: none;
	}
	.messageview-msglist > row.messageview-editing,
	.messageview-msglist > row.messageview-replyingto {
		transition: none;
		background-color: alpha(@accent_bg_color, 0.25);
		background-size: 18px;
		background-repeat: no-repeat;
		background-position: calc(100% - 5px) 5px;
	}
	.messageview-msglist > row.messageview-edited:hover,
	.messageview-msglist > row.messageview-replyingto:hover {
		background-color: alpha(@accent_bg_color, 0.45);
	}
	.messageview-msglist > row.messageview-replyingto {
		background-image: -gtk-icontheme("mail-reply-sender");
	}
	.messageview-msglist > row.messageview-editing {
		background-image: -gtk-icontheme("document-edit");
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
		mrelated: make(map[matrix.EventID]matrix.EventID),

		onTitle: func(string) {},
		ctx:     gtkutil.WithVisibility(ctx, msgList),
		name:    name,

		parent: parent,
		roomID: roomID,
	}

	page.list.SetSelectionMode(gtk.SelectionNone)
	msgListCSS(page.list)

	// This sorting is a HUGE issue. It's a really, really big issue, actually.
	// Right now, we're checking whether or not a message should be collapsed by
	// checking the last message. The problem is that sorting can kick that
	// order off AFTER creation, so in a way, to properly fix this issue, we'll
	// need to refactor the code so that the message state is COMPLETELY
	// separated, and then we insert a hollow Row, and then we initialize the
	// content BASED ON what's sorted after the fact. Quite hairy.

	// TODO: decouple message component from state.
	// TODO: lazy rendering message component inside ListBoxRow.
	// TODO: API to re-render the message and toggle between compact and full.

	// Sort messages according to the timestamp.
	// msgList.SetSortFunc(func(r1, r2 *gtk.ListBoxRow) int {
	// 	m1, ok1 := page.messages[matrix.EventID(r1.Name())]
	// 	m2, ok2 := page.messages[matrix.EventID(r2.Name())]
	// 	if !ok1 || !ok2 {
	// 		return 0
	// 	}
	// 	if m1.sent < m2.sent {
	// 		return -1
	// 	}
	// 	if m1.sent == m2.sent {
	// 		return 0
	// 	}
	// 	return 1 // t1 > t2
	// })

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

	page.extra = newExtraRevealer()
	page.extra.SetVAlign(gtk.AlignEnd)

	overlay := gtk.NewOverlay()
	overlay.SetVExpand(true)
	overlay.SetChild(page.scroll)
	overlay.AddOverlay(page.extra)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(overlay)
	box.Append(page.Composer)
	rhsCSS(box)

	// main widget
	page.Widgetter = box

	gtkutil.MapSubscriber(page, func() func() {
		return parent.client.SubscribeTimeline(roomID, func(r *event.RawEvent) {
			glib.IdleAdd(func() { page.OnRoomEvent(r) })
		})
	})

	gtkutil.MapSubscriber(page, func() func() {
		return parent.client.SubscribeRoom(roomID, event.TypeTyping, func(e event.Event) {
			ev := e.(event.TypingEvent)
			if len(ev.UserID) == 0 {
				page.extra.Clear()
				return
			}

			names := make([]string, len(ev.UserID))
			for i, id := range ev.UserID {
				author := mauthor.Markup(parent.client, roomID, id, mauthor.WithMinimal())
				names[i] = "<b>" + author + "</b>"
			}

			msg := locale.Plural(ctx, names, "is typing...", "are typing...")
			page.extra.SetMarkup(msg)
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

func (p *Page) lastRow() *gtk.ListBoxRow {
	w := p.list.LastChild()
	if w != nil {
		return w.(*gtk.ListBoxRow)
	}
	return nil
}

// LastMessage satisfies MessageViewer.
func (p *Page) LastMessage() message.Message {
	lastRow := p.lastRow()
	if lastRow == nil {
		return nil
	}

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
	if !p.IsActive() || !p.scroll.IsBottomed() {
		return
	}

	lastRow := p.lastRow()
	if lastRow == nil {
		return
	}

	client := gotktrix.FromContext(p.ctx.Take())
	latest := matrix.EventID(lastRow.Name())

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

	lastRow := p.lastRow()

	for lastRow.Index() >= gotktrix.TimelimeLimit {
		row := p.list.RowAtIndex(0)
		if row == nil {
			continue
		}

		id := matrix.EventID(row.Name())
		delete(p.messages, id)

		p.list.Remove(row)

		for k, relatesTo := range p.mrelated {
			if relatesTo == id {
				delete(p.mrelated, k)
			}
		}
	}
}

func (p *Page) onRoomEvent(raw *event.RawEvent) {
	id := raw.ID
	relatesToID := relatesTo(raw)

	if relatesToID != "" {
		rl, ok := p.messages[relatesToID]
		if !ok {
			if rel := p.mrelated[relatesToID]; rel != "" {
				rl, ok = p.messages[rel]
			}
		}
		if ok {
			// Register this event as a related event.
			p.mrelated[raw.ID] = rl.msg.Event().ID()
			// Trigger the message's callback.
			rl.msg.OnRelatedEvent(gotktrix.WrapEventBox(raw))
			return
		}
		// Treat as a new message.
	}

	m := message.NewCozyMessage(p.parent.ctx, p, raw)

	row := gtk.NewListBoxRow()
	row.SetName(string(id))
	row.SetChild(m)

	p.messages[id] = messageRow{
		msg:  m,
		row:  row,
		sent: raw.OriginServerTime,
	}

	p.list.Append(row)
}

// relatesTo returns the event ID that the given raw event is supposed to edit,
// or an empty string if it does not edit anything.
func relatesTo(raw *event.RawEvent) matrix.EventID {
	if raw.Type == event.TypeRoomRedaction {
		return raw.Redacts
	}

	var body struct {
		RelatesTo struct {
			EventID matrix.EventID `json:"event_id"`
		} `json:"m.relates_to"`
	}

	if err := json.Unmarshal(raw.Content, &body); err != nil {
		return ""
	}

	return body.RelatesTo.EventID
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

		for i := range events {
			i := i // copy for referencing
			glib.IdleAdd(func() { p.onRoomEvent(&events[i]) })
		}

		glib.IdleAdd(func() {
			p.loaded = true
			p.scroll.ScrollToBottom()
			done()
		})
	}()
}

// Edit triggers the input composer to edit an existing message.
func (p *Page) Edit(eventID matrix.EventID) {
	if p.replyingTo != "" {
		// Stop replying.
		p.ReplyTo("")
	}

	p.singleMessageState(
		eventID, &p.editing, p.Composer.Edit,
		"messageview-editing",
	)
}

// ReplyTo sets the event ID that the user wants to reply to.
func (p *Page) ReplyTo(eventID matrix.EventID) {
	if p.editing != "" {
		// Stop editing.
		p.Edit("")
	}

	p.singleMessageState(
		eventID, &p.replyingTo, p.Composer.ReplyTo,
		"messageview-replyingto",
	)
}

func (p *Page) singleMessageState(
	eventID matrix.EventID,
	field *matrix.EventID, set func(matrix.EventID), class string) {

	if *field != "" {
		r, ok := p.messages[*field]
		if ok {
			r.row.RemoveCSSClass(class)
		}
		*field = ""
	}

	mr, ok := p.messages[eventID]
	if !ok {
		set("")
		return
	}

	set(eventID)
	mr.row.AddCSSClass(class)

	*field = eventID
}
