package messageview

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/compose"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/components/autoscroll"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

type messageKey string

const (
	messageKeyEventPrefix = "event"
	messageKeyLocalPrefix = "local"
)

func messageKeyRow(row *gtk.ListBoxRow) messageKey {
	if row == nil {
		return ""
	}

	name := row.Name()
	if !strings.Contains(name, ":") {
		log.Panicf("row name %q not a messageKey", name)
	}

	return messageKey(name)
}

// messageKeyEvent returns the messageKey for a server event.
func messageKeyEvent(event event.RoomEvent) messageKey {
	return messageKeyEventID(event.RoomInfo().ID)
}

// messageKeyEventID returns the messageKey for a server event ID.
func messageKeyEventID(event matrix.EventID) messageKey {
	return messageKey(messageKeyEventPrefix + ":" + string(event))
}

var messageKeyLocalInc = new(uint64)

// messageKeyLocal creates a new local messageKey that will never collide with
// server events.
func messageKeyLocal() messageKey {
	inc := atomic.AddUint64(messageKeyLocalInc, 1)
	return messageKey(fmt.Sprintf("%s:%d-%d", messageKeyLocalPrefix, time.Now().UnixNano(), inc))
}

func (k messageKey) parts() (typ, val string) {
	parts := strings.SplitN(string(k), ":", 2)
	if len(parts) != 2 {
		log.Panicf("invalid messageKey %q", parts)
	}
	return parts[0], parts[1]
}

// EventID takes the event ID from the messae key. If the key doesn't hold an
// event ID, then it panics.
func (k messageKey) EventID() matrix.EventID {
	typ, val := k.parts()
	if typ != messageKeyEventPrefix {
		panic("EventID called on non-event message key")
	}
	return matrix.EventID(val)
}

func (k messageKey) IsEvent() bool {
	typ, _ := k.parts()
	return typ == messageKeyEventPrefix
}

func (k messageKey) IsLocal() bool {
	typ, _ := k.parts()
	return typ == messageKeyLocalPrefix
}

// Page describes a tab page, which is a single message view. It satisfies teh
// MessageViewer interface.
type Page struct {
	gtk.Widgetter
	Composer *compose.Composer

	main *adaptive.LoadablePage
	box  *gtk.Box

	extra *extraRevealer

	scroll *autoscroll.Window
	list   *gtk.ListBox
	// TODO: it might be better to refactor these maps into a map of only an
	// event object that simultaneously has a linked anchor. This way, there's
	// no need to keep two separate maps, and there's no need to handle small
	// pieces of events in separate places.
	messages map[messageKey]messageRow
	mrelated map[matrix.EventID]matrix.EventID // keep track of reactions

	name    string
	onTitle func(title string)
	ctx     gtkutil.Canceller

	parent *View
	pager  *gotktrix.RoomPaginator
	roomID matrix.RoomID

	editing    matrix.EventID
	replyingTo matrix.EventID

	loaded bool
}

type messageRow struct {
	row  *gtk.ListBoxRow
	ev   event.RoomEvent
	body message.Message
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
		background-color: alpha(@theme_selected_bg_color, 0.25);
		background-size: 18px;
		background-repeat: no-repeat;
		background-position: calc(100% - 5px) 5px;
	}
	.messageview-msglist > row.messageview-edited:hover,
	.messageview-msglist > row.messageview-replyingto:hover {
		background-color: alpha(@theme_selected_bg_color, 0.45);
	}
	.messageview-msglist > row.messageview-replyingto {
		background-image: -gtk-icontheme("mail-reply-sender");
	}
	.messageview-msglist > row.messageview-editing {
		background-image: -gtk-icontheme("document-edit");
	}
`)

var rhsCSS = cssutil.Applier("messageview-rhs", `
	.messageview-rhs .messageview-box {
		background-image: linear-gradient(to top, @theme_base_color 0px, transparent 40px);
	}
`)

// maxFetch is the number of events to initially display. Keep it low so loading
// isn't as slow.
const maxFetch = 35

// NewPage creates a new page.
func NewPage(ctx context.Context, parent *View, roomID matrix.RoomID) *Page {
	name, _ := parent.client.Offline().RoomName(roomID)

	msgList := gtk.NewListBox()

	page := Page{
		list:     msgList,
		messages: make(map[messageKey]messageRow),
		mrelated: make(map[matrix.EventID]matrix.EventID),

		onTitle: func(string) {},
		ctx:     gtkutil.WithVisibility(ctx, msgList),
		name:    name,

		parent: parent,
		pager:  parent.client.RoomPaginator(roomID, maxFetch),
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

	// TODO: this is still subtly buggy. We're currently assuming that the
	// sorted order stays the same for all messages, but that assumption is
	// incorrect. We might want to use GListModel for this.

	// Sort messages according to the timestamp.
	msgList.SetSortFunc(func(r1, r2 *gtk.ListBoxRow) int {
		m1, ok1 := page.messages[messageKeyRow(r1)]
		m2, ok2 := page.messages[messageKeyRow(r2)]
		if !ok1 || !ok2 {
			return 0
		}

		t1 := m1.ev.RoomInfo().OriginServerTime
		t2 := m2.ev.RoomInfo().OriginServerTime
		if t1 < t2 {
			return -1
		}
		if t1 == t2 {
			return 0
		}
		return 1 // t1 > t2
	})

	innerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	innerBox.Append(newLoadMore(page.loadMore))
	innerBox.Append(page.list)
	innerBox.SetFocusChild(page.list)

	vp := gtk.NewViewport(nil, nil)
	vp.SetVScrollPolicy(gtk.ScrollNatural)
	vp.SetScrollToFocus(true)
	vp.SetChild(innerBox)

	page.scroll = autoscroll.NewWindow()
	page.scroll.SetPropagateNaturalWidth(true)
	page.scroll.SetPropagateNaturalHeight(true)
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

	page.box = gtk.NewBox(gtk.OrientationVertical, 0)
	page.box.Append(overlay)
	page.box.Append(page.Composer)
	page.box.SetFocusChild(page.Composer)
	page.box.AddCSSClass("messageview-box")

	page.main = adaptive.NewLoadablePage()
	page.main.SetChild(page.box)
	rhsCSS(page.main)

	// main widget
	page.Widgetter = page.main

	page.ctx.OnRenew(func(context.Context) func() {
		return parent.client.SubscribeTimeline(roomID, func(r event.RoomEvent) {
			glib.IdleAdd(func() { page.OnRoomEvent(r) })
		})
	})

	page.ctx.OnRenew(func(context.Context) func() {
		client := gotktrix.FromContext(ctx)
		return client.SubscribeRoom(roomID, event.TypeTyping, func(e event.Event) {
			ev, ok := e.(*event.TypingEvent)
			if !ok || len(ev.UserID) == 0 {
				page.extra.Clear()
				return
			}

			// 3 UserIDs max.
			if len(ev.UserID) > 3 {
				ev.UserID = ev.UserID[:3]
			}

			names := make([]string, len(ev.UserID))
			for i, id := range ev.UserID {
				author := mauthor.Markup(client, roomID, id, mauthor.WithMinimal())
				names[i] = "<b>" + author + "</b>"
			}

			var msg string
			p := locale.FromContext(ctx)

			switch len(names) {
			case 0:
				glib.IdleAdd(func() { page.extra.SetMarkup("") })
				return
			case 1:
				msg = p.Sprintf("%s is typing...", names[0])
			case 2:
				msg = p.Sprintf("%s and %s are typing...", names[0], names[1])
			case 3:
				msg = p.Sprintf("%s, %s and %s are typing...", names[0], names[1], names[2])
			default:
				msg = p.Sprintf("Several people are typing...")
			}

			glib.IdleAdd(func() { page.extra.SetMarkup(msg) })
		})
	})

	page.scroll.OnBottomed(func(bottomed bool) {
		if bottomed {
			// Mark the latest message as read everytime the user scrolls down
			// to the bottom.
			page.MarkAsRead()
		}
	})

	page.ctx.OnRenew(func(ctx context.Context) func() {
		w := app.Window(ctx)
		h := w.Connect("notify::is-active", func() {
			if w.IsActive() && page.scroll.IsBottomed() {
				// Mark the page as read if the window is focused and our scroll
				// is at the bottom.
				page.MarkAsRead()
			}
		})
		return func() {
			w.HandlerDisconnect(h)
		}
	})

	// Focus and forward all typing events from the message list to the input
	// composer.
	gtkutil.ForwardTyping(page.list, page.Composer.Input())

	return &page
}

// IsActive returns true if this page is the one the user is viewing.
func (p *Page) IsActive() bool {
	return p.parent.current != nil && p.parent.current.roomID == p.roomID
}

// OnTitle subscribes to the page's title changes.
func (p *Page) OnTitle(f func(string)) {
	p.onTitle = f
	f(p.RoomName())
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

// RoomTopic returns the room's topic name.
func (p *Page) RoomTopic() string {
	client := gotktrix.FromContext(p.ctx.Take()).Offline()

	e, _ := client.RoomState(p.roomID, event.TypeRoomTopic, "")
	if e != nil {
		nameEvent := e.(*event.RoomTopicEvent)
		return nameEvent.Topic
	}

	return ""
}

// OnRoomEvent is called on every room event belonging to this room.
func (p *Page) OnRoomEvent(ev event.RoomEvent) {
	if ev.RoomInfo().RoomID != p.roomID {
		return
	}

	key := p.onRoomEvent(ev)

	r, ok := p.messages[key]
	if ok {
		r.body.LoadMore()
	}

	p.clean()
	p.MarkAsRead()
}

// FocusLatestUserEventID returns the latest valid event ID of the current user
// in the room or an empty string if none. It implements compose.Controller.
func (p *Page) FocusLatestUserEventID() matrix.EventID {
	userID := gotktrix.FromContext(p.ctx.Take()).UserID

	row := p.list.LastChild().(*gtk.ListBoxRow)
	for row != nil {
		m, ok := p.messages[messageKey(row.Name())]
		if ok && m.ev.RoomInfo().Sender == userID {
			m.row.GrabFocus()
			return m.ev.RoomInfo().ID
		}
		// This repeats until index is -1, at which the loop will break.
		row = p.list.RowAtIndex(row.Index() - 1)
	}

	return ""
}

// lastRow returns the list's last row.
func (p *Page) lastRow() *gtk.ListBoxRow {
	w := p.list.LastChild()
	if w != nil {
		return w.(*gtk.ListBoxRow)
	}
	return nil
}

// MarkAsRead marks the room as read.
func (p *Page) MarkAsRead() {
	if !p.IsActive() || !p.scroll.IsBottomed() || !app.Window(p.ctx.Take()).IsActive() {
		return
	}

	lastRow := p.lastRow()
	if lastRow == nil {
		return
	}

	// Set the message view's focus to the right last message.
	p.list.SetFocusChild(lastRow)

	client := gotktrix.FromContext(p.ctx.Take())
	roomID := p.roomID

	go func() {
		// Pull the events from the room directly from the state. We do this
		// because the room sometimes coalesce events together.
		latest, _ := client.State.LatestInTimeline(roomID, "")
		if latest == nil {
			return
		}

		if err := client.MarkRoomAsRead(roomID, latest.RoomInfo().ID); err != nil {
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

	for lastRow.Index() >= maxFetch*2 {
		row := p.list.RowAtIndex(0)
		if row == nil {
			return
		}

		p.list.Remove(row)

		id := messageKeyRow(row)
		delete(p.messages, id)

		if id.IsEvent() {
			for k, relatesTo := range p.mrelated {
				if relatesTo == id.EventID() {
					delete(p.mrelated, k)
				}
			}
		}
	}
}

// MessageMark is a struct that marks a specific position of a message. It is
// not guaranteed to be immutable while it is held, and the user should treat it
// as an opaque structure.
type MessageMark struct {
	row *gtk.ListBoxRow
}

// AddSendingMessage adds the given message into the page and returns the row.
// The user must call BindSendingMessage afterwards to ensure that the added
// message is merged with the synchronized one.
func (p *Page) AddSendingMessage(ev event.RoomEvent) interface{} {
	key := messageKeyLocal()

	row := gtk.NewListBoxRow()
	row.SetName(string(key))
	row.AddCSSClass("messageview-messagerow")
	row.AddCSSClass("messageview-usermessage")

	p.setMessage(key, messageRow{
		row: row,
		ev:  ev,
	})

	p.messages[key].body.SetBlur(true)
	return key
}

// BindSendingMessage is used after the sending message has been sent through
// the backend, and that an event ID is returned. The page will try to match the
// message up with an existing event.
func (p *Page) BindSendingMessage(mark interface{}, evID matrix.EventID) (replaced bool) {
	key, ok := mark.(messageKey)
	if !ok {
		return false
	}

	msg, ok := p.messages[key]
	if !ok {
		return false
	}
	delete(p.messages, key)

	eventKey := messageKeyEventID(evID)
	// Check if the message has been synchronized before it's replied.
	if _, ok := p.messages[eventKey]; ok {
		// Store the index which will be the next message once we remove the
		// current one.
		ix := msg.row.Index()
		log.Println("message bound after arrival from server, deleting")
		// Yes, so replace our sending message.
		p.list.Remove(msg.row)
		// Reset the message that fills the gap. This isn't very important, so
		// we ignore the returned boolean.
		p.resetMessageIx(ix)
		// Just use the synced message.
		return true
	}

	// Not replaced yet, so we arrived first. Place the message in.
	info := msg.ev.RoomInfo()
	info.ID = evID
	p.messages[eventKey] = msg

	msg.row.SetName(string(eventKey))
	msg.body.SetBlur(false)
	log.Println("message bound from API")

	return false
}

func (p *Page) relatedEvent(relatesTo matrix.EventID) (messageRow, bool) {
	for relatesTo != "" {
		r, ok := p.messages[messageKeyEventID(relatesTo)]
		if ok {
			return r, true
		}
		relatesTo = p.mrelated[relatesTo]
	}
	return messageRow{}, false
}

func (p *Page) onRoomEvent(ev event.RoomEvent) (key messageKey) {
	key = messageKeyEvent(ev)

	if relatesToID := relatesTo(ev); relatesToID != "" {
		r, ok := p.relatedEvent(relatesToID)
		if ok {
			// Register this event as a related event.
			p.mrelated[ev.RoomInfo().ID] = relatesToID
			// Trigger the message's callback.
			r.body.OnRelatedEvent(ev)
			return
		}
		// Treat as a new message.
	}

	// Ensure that there isn't already a message with the same ID, which might
	// happen if this is a message that we sent.
	if existing, ok := p.messages[key]; ok {
		existing.ev = ev
		log.Println("message arrived from server with existing ID")
		p.setMessage(key, existing)
		return
	}

	// Bug with sequence:
	//  1. message bound from API
	//  2. message arrived from server with existing ID
	// Seems like on step 2, the message is incorrectly collapsed.

	row := gtk.NewListBoxRow()
	row.SetName(string(key))
	row.AddCSSClass("messageview-messagerow")

	// Prematurely initialize this with an empty body for the sort function to
	// work.
	p.setMessage(key, messageRow{
		row: row,
		ev:  ev,
	})
	return
}

func (p *Page) setMessage(key messageKey, msg messageRow) {
	p.messages[key] = msg

	// Appending or prepending to this will cause the ListBox to sort the row
	// for us. Hopefully, if we hint the position, ListBox won't try to sort too
	// many times.
	if msg.row.Index() == -1 {
		p.list.Append(msg.row)
	}

	ix := msg.row.Index()

	// We can now reliably create a new message with the right previous message.
	// Note that this will break once the message's author starts mutating by
	// being either modified or removed.
	p.resetMessageIx(ix)

	// If we're inserting this message before an existing one, then we should
	// recreate the one after as well, in case it belongs to a different author.
	if ix < len(p.messages)-1 {
		p.resetMessage(p.keyAtIndex(ix+1), msg)
	} else {
		// Set default focus to the last row.
		p.list.SetFocusChild(msg.row)
	}
}

func (p *Page) resetMessageIx(ix int) bool {
	return p.resetMessage(
		p.keyAtIndex(ix),
		p.rowAtIndex(ix-1),
	)
}

func (p *Page) resetMessage(key messageKey, before messageRow) bool {
	msg, ok := p.messages[key]
	if !ok {
		return false
	}

	if before.body != nil {
		log.Printf(
			"for message by %q, found %q (ours=%q) before",
			msg.ev.RoomInfo().Sender, before.body.Event().RoomInfo().Sender, before.ev.RoomInfo().Sender)
	}

	// Recreate the body if the raw events don't match.
	if msg.body == nil || !eventEq(msg.ev, msg.body.Event()) {
		msg.body = message.NewCozyMessage(p.parent.ctx, p, msg.ev, before.body)
		p.messages[key] = msg

		msg.row.SetChild(msg.body)
	}

	return true
}

func eventEq(e1, e2 event.RoomEvent) bool {
	r1 := e1.RoomInfo()
	r2 := e2.RoomInfo()
	// Only compare if both of the messages have the Raw field.
	if r1.Raw == nil || r2.Raw == nil {
		return false
	}
	return r1.OriginServerTime == r2.OriginServerTime && r1.ID == r2.ID
}

// relatesTo returns the event ID that the given raw event is supposed to edit,
// or an empty string if it does not edit anything.
func relatesTo(ev event.RoomEvent) matrix.EventID {
	// It's best to keep this in sync with mcontent/content.go's OnRelatedEvent.
	// We don't need to deep-inspect the JSON.
	switch ev := ev.(type) {
	case *event.RoomRedactionEvent:
		return ev.Redacts
	case *m.ReactionEvent:
		return ev.RelatesTo.EventID
	case *event.RoomMessageEvent:
		var relatesTo struct {
			EventID matrix.EventID `json:"event_id"`
		}
		json.Unmarshal(ev.RelatesTo, &relatesTo)
		return relatesTo.EventID
	default:
		return ""
	}
}

// rowAtIndex gets the messageRow at the given index. A zero-value is returned
// if it's not found.
func (p *Page) rowAtIndex(i int) messageRow {
	key := p.keyAtIndex(i)
	return p.messages[key]
}

func (p *Page) keyAtIndex(i int) messageKey {
	return messageKeyRow(p.list.RowAtIndex(i))
}

// Load asynchronously loads the page. The given callback is called once the
// page finishes loading.
func (p *Page) Load(done func()) {
	if p.loaded {
		done()
		return
	}
	p.loaded = true

	p.ctx.Renew()
	ctx := p.ctx.Take()
	client := p.parent.client.WithContext(ctx)

	fetchName := p.name == ""

	load := func(events []event.RoomEvent) {
		p.main.SetChild(p.box)
		p.scroll.ScrollToBottom()
		p.addBulkEvents(events)
		done()
	}

	// We can rely on this comparison to directly call Paginate on the main
	// thread, since it means the state events will be used instead. To ensure
	// that no API calls are done, we can give it a cancelled context and
	// fallback to fetching from the API asynchronously.
	if maxFetch < gotktrix.TimelimeLimit {
		events, err := p.pager.Paginate(gotktrix.Cancelled())
		if err == nil {
			load(events)
			return
		}
	}

	p.main.SetLoading()

	gtkutil.Async(ctx, func() func() {
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

		events, err := p.pager.Paginate(ctx)
		if err != nil {
			app.Error(ctx, err)
			return func() {
				p.main.SetError(err)
				done()
			}
		}

		return func() { load(events) }
	})
}

func (p *Page) loadMore(done paginateDoneFunc) {
	ctx := p.ctx.Take()

	gtkutil.Async(ctx, func() func() {
		events, err := p.pager.Paginate(ctx)
		if err != nil {
			return func() { done(true, err) }
		}

		return func() {
			p.scroll.SetScrollLocked(true)
			defer p.scroll.SetScrollLocked(false)

			p.addBulkEvents(events)

			// TODO: check for hasMore.
			done(true, nil)
		}
	})
}

func (p *Page) addBulkEvents(events []event.RoomEvent) {
	keys := make([]messageKey, len(events))
	// Require old messages first, so cozy mode works properly.
	for i, ev := range events {
		keys[i] = p.onRoomEvent(ev)
	}

	// Load the newest messages first so it doesn't screw up scrolling as
	// hard.
	for i := len(keys) - 1; i >= 0; i-- {
		r, ok := p.messages[keys[i]]
		if ok {
			r.body.LoadMore()
		}
	}
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
		r, ok := p.messages[messageKeyEventID(*field)]
		if ok {
			r.row.RemoveCSSClass(class)
		}
		*field = ""
	}

	mr, ok := p.messages[messageKeyEventID(eventID)]
	if !ok {
		if rel := p.mrelated[eventID]; rel != "" {
			mr, ok = p.messages[messageKeyEventID(rel)]
		}
	}
	if !ok {
		set("")
		return
	}

	set(eventID)
	mr.row.AddCSSClass(class)

	*field = eventID
}
