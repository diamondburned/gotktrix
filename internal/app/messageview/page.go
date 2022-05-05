package messageview

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/autoscroll"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/compose"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
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

	// moreMsgBar is the bar on top that pops up when there are new unread
	// messages in the current room.
	moreMsgBar  *moreMessageBar
	markReadBtn *gtk.Button

	scroll *autoscroll.Window
	list   *gtk.ListBox
	// TODO: it might be better to refactor these maps into a map of only an
	// event object that simultaneously has a linked anchor. This way, there's
	// no need to keep two separate maps, and there's no need to handle small
	// pieces of events in separate places.
	messages map[messageKey]messageRow
	mrelated map[matrix.EventID]matrix.EventID // keep track of reactions

	// extra is the bottom popup for typing indicators and etc.
	extra *extraRevealer

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
	// these fields determine the state of the fields after it.
	row    *gtk.ListBoxRow
	ev     event.RoomEvent
	custom bool
	// these fields are changed depending on the above fields.
	body message.Message
	// before tracks the event before so we can invalidate it if we insert a new
	// one before.
	before matrix.EventID
}

var _ message.MessageViewer = (*Page)(nil)

var msgListCSS = cssutil.Applier("messageview-msglist", `
	.messageview-msglist {
		background: none;
		margin-bottom: .8em; /* for the extraRevealer */
	}
	.messageview-msglist > row {
		padding: 0;
		transition: linear 150ms background-color;
		background: none;
		background-image: none;
		background-color: transparent;
	}
	.messageview-msglist > row:focus,
	.messageview-msglist > row:hover {
		transition: none;
	}
	.messageview-msglist > row:focus {
		background-color: alpha(@theme_fg_color, 0.125);
	}
	.messageview-msglist > row:hover {
		background-color: alpha(@theme_fg_color, 0.075);
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
const maxFetch = 40

var messageviewEvents = []event.Type{
	event.TypeTyping,
	m.FullyReadEventType,
}

// NewPage creates a new page.
func NewPage(ctx context.Context, parent *View, roomID matrix.RoomID) *Page {
	name, _ := parent.client.Offline().RoomName(roomID)

	p := Page{
		messages: make(map[messageKey]messageRow),
		mrelated: make(map[matrix.EventID]matrix.EventID),

		onTitle: func(string) {},
		name:    name,

		parent: parent,
		pager:  parent.client.RoomPaginator(roomID, maxFetch),
		roomID: roomID,
	}

	p.list = gtk.NewListBox()
	p.list.SetSelectionMode(gtk.SelectionNone)
	msgListCSS(p.list)

	p.ctx = gtkutil.WithVisibility(ctx, p.list)

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
	p.list.SetSortFunc(func(r1, r2 *gtk.ListBoxRow) int {
		m1, ok1 := p.messages[messageKeyRow(r1)]
		m2, ok2 := p.messages[messageKeyRow(r2)]
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
	innerBox.Append(newLoadMore(p.loadMore))
	innerBox.Append(p.list)
	innerBox.SetFocusChild(p.list)

	p.scroll = autoscroll.NewWindow()
	p.scroll.SetPropagateNaturalWidth(true)
	p.scroll.SetPropagateNaturalHeight(true)
	p.scroll.SetVExpand(true)
	p.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	p.scroll.SetChild(innerBox)

	vp := p.scroll.Viewport()
	vp.SetScrollToFocus(true)

	// Bind the scrolled window for automatic scrolling.
	p.list.SetAdjustment(p.scroll.VAdjustment())

	p.Composer = compose.New(ctx, &p, roomID)

	p.extra = newExtraRevealer()
	p.extra.SetVAlign(gtk.AlignEnd)

	p.moreMsgBar = newMoreMessageBar(ctx, p.roomID)
	p.moreMsgBar.SetVAlign(gtk.AlignStart)

	p.markReadBtn = p.moreMsgBar.AddButton(locale.S(ctx, "Mark as read"))
	p.markReadBtn.ConnectClicked(func() { p.MarkAsRead() })

	overlay := gtk.NewOverlay()
	overlay.SetVExpand(true)
	overlay.SetChild(p.scroll)
	overlay.AddOverlay(p.extra)
	overlay.AddOverlay(p.moreMsgBar)

	p.box = gtk.NewBox(gtk.OrientationVertical, 0)
	p.box.Append(overlay)
	p.box.Append(p.Composer)
	p.box.SetFocusChild(p.Composer)
	p.box.AddCSSClass("messageview-box")

	p.main = adaptive.NewLoadablePage()
	p.main.SetChild(p.box)
	rhsCSS(p.main)

	// main widget
	p.Widgetter = p.main

	p.ctx.OnRenew(func(context.Context) func() {
		return parent.client.SubscribeTimeline(roomID, func(r event.RoomEvent) {
			glib.IdleAdd(func() { p.OnRoomEvent(r) })
		})
	})

	p.ctx.OnRenew(func(context.Context) func() {
		client := gotktrix.FromContext(ctx)
		return client.SubscribeRoomEvents(roomID, messageviewEvents, func(e event.Event) {
			glib.IdleAdd(func() {
				switch e := e.(type) {
				case *event.TypingEvent:
					p.onTypingEvent(e)
				case *m.FullyReadEvent:
					p.moreMsgBar.Invalidate()
				}
			})
		})
	})

	// Mark the latest message as read everytime the user scrolls down to the
	// bottom.
	p.scroll.OnBottomed(p.OnScrollBottomed)

	p.ctx.OnRenew(func(ctx context.Context) func() {
		w := app.GTKWindowFromContext(ctx)
		h := w.NotifyProperty("is-active", p.OnScrollBottomed)
		return func() { w.HandlerDisconnect(h) }
	})

	// Focus and forward all typing events from the message list to the input
	// composer.
	gtkutil.ForwardTyping(p.list, p.Composer.Input())

	return &p
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

func (p *Page) onTypingEvent(ev *event.TypingEvent) {
	if len(ev.UserID) == 0 {
		p.extra.Clear()
		return
	}

	// 3 UserIDs max.
	if len(ev.UserID) > 3 {
		ev.UserID = ev.UserID[:3]
	}

	client := gotktrix.FromContext(p.ctx.Take())

	names := make([]string, len(ev.UserID))
	for i, id := range ev.UserID {
		author := mauthor.Markup(client, p.roomID, id, mauthor.WithMinimal())
		names[i] = "<b>" + author + "</b>"
	}

	sprintf := locale.FromContext(p.ctx.Take()).Sprintf

	switch len(names) {
	case 0:
		p.extra.SetMarkup("")
	case 1:
		p.extra.SetMarkup(sprintf("%s is typing...", names[0]))
	case 2:
		p.extra.SetMarkup(sprintf("%s and %s are typing...", names[0], names[1]))
	case 3:
		p.extra.SetMarkup(sprintf("%s, %s and %s are typing...", names[0], names[1], names[2]))
	default:
		p.extra.SetMarkup(sprintf("Several people are typing..."))
	}
}

// FocusLatestUserEventID returns the latest valid event ID of the current user
// in the room or an empty string if none. It implements compose.Controller.
func (p *Page) FocusLatestUserEventID() matrix.EventID {
	row, ok := p.latestUserMessage()
	if !ok {
		return ""
	}

	row.row.GrabFocus()
	return row.ev.RoomInfo().ID
}

func (p *Page) latestUserMessage() (messageRow, bool) {
	userID := gotktrix.FromContext(p.ctx.Take()).UserID

	row := p.list.LastChild().(*gtk.ListBoxRow)
	for row != nil {
		key := messageKey(row.Name())
		if key.IsEvent() {
			m, ok := p.messages[key]
			if ok && m.ev.RoomInfo().Sender == userID {
				return m, true
			}
		}

		// This repeats until index is -1, at which the loop will break.
		row = p.list.RowAtIndex(row.Index() - 1)
	}

	return messageRow{}, false
}

// lastRow returns the list's last row.
func (p *Page) lastRow() *gtk.ListBoxRow {
	w := p.list.LastChild()
	if w != nil {
		return w.(*gtk.ListBoxRow)
	}
	return nil
}

// OnScrollBottomed marks the room as read if the page is focused, the window
// the page is in are focused, and the user is currently scrolled to the bottom.
func (p *Page) OnScrollBottomed() {
	row, ok := p.messages[messageKeyRow(p.lastRow())]
	if !ok {
		return
	}

	userID := gotktrix.FromContext(p.ctx.Take()).UserID

	// Permit marking as read if the latest sent message is our message, since
	// clearly we've read that message. Otherwise, do checks as normal.
	if userID != row.ev.RoomInfo().Sender {
		if !p.IsActive() || !p.scroll.IsBottomed() || !app.IsActive(p.ctx.Take()) {
			return
		}
	}

	p.MarkAsRead()
}

// MarkAsRead marks the room as read.
func (p *Page) MarkAsRead() {
	lastRow := p.lastRow()
	if lastRow == nil {
		return
	}

	// Set the message view's focus to the right last message.
	p.list.SetFocusChild(lastRow)

	client := gotktrix.FromContext(p.ctx.Take())
	roomID := p.roomID

	p.markReadBtn.SetSensitive(false)
	done := func(hide bool) {
		if hide {
			p.moreMsgBar.Hide()
			p.markReadBtn.SetSensitive(true)
		}
	}

	gtkutil.Async(p.ctx.Take(), func() func() {
		// Pull the events from the room directly from the state. We do this
		// because the room sometimes coalesce events together.
		latest, _ := client.State.LatestInTimeline(roomID, "")
		if latest == nil {
			return func() { done(true) }
		}

		if err := client.MarkRoomAsRead(roomID, latest.RoomInfo().ID); err != nil {
			// No need to interrupt the user for this.
			log.Println("failed to mark room as read:", err)
			return func() { done(false) }
		}

		return func() { done(true) }
	})
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

// Future note: an interface{} is returned here to prevent cyclical dependency.
// It makes sense to return one here, since it forces the user to not make any
// assumptions about the returned value, forcing to treat it as an opaque value.

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

// SetSendingMessageBody sets the body of the message to the given widget. This
// is useful for sending actions that require displaying some information to the
// user.
func (p *Page) AddSendingMessageCustom(ev event.RoomEvent, w gtk.Widgetter) interface{} {
	key := messageKeyLocal()

	row := gtk.NewListBoxRow()
	row.SetName(string(key))
	row.SetChild(w)
	row.AddCSSClass("messageview-messagerow")
	row.AddCSSClass("messageview-usermessage")
	row.AddCSSClass("messageview-usermessage-custom")

	p.setMessage(key, messageRow{
		row:    row,
		ev:     ev,
		custom: true,
	})

	return key
}

// StopSendingMessage removes the sending message with the given mark.
func (p *Page) StopSendingMessage(mark interface{}) bool {
	key, ok := mark.(messageKey)
	if !ok {
		return false
	}

	msg, ok := p.messages[key]
	if !ok {
		return false
	}

	delete(p.messages, key)
	p.list.Remove(msg.row)

	return true
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
	if old, ok := p.messages[eventKey]; ok {
		// Yes, so replace our sending message.
		p.list.Remove(msg.row)
		// The synchronized message is going to be compacted when it saw that
		// our fake message is already there, so we have to invalidate it after
		// removing the fake one.
		old.body = nil
		p.setMessage(eventKey, old)
		// Just use the synced message.
		return true
	}

	// Not replaced yet, so we arrived first. Place the message in.
	info := msg.ev.RoomInfo()
	info.ID = evID

	if msg.custom {
		msg.custom = false
	} else {
		msg.body.SetBlur(false)
	}

	msg.row.SetName(string(eventKey))
	p.messages[eventKey] = msg

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

// OnRoomEvent is called on every room timeline event belonging to this room.
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
	p.OnScrollBottomed()
}

func (p *Page) onRoomEvent(ev event.RoomEvent) (key messageKey) {
	key = messageKeyEvent(ev)

	if relatesToID := relatesTo(ev); relatesToID != "" {
		r, ok := p.relatedEvent(relatesToID)
		if ok && r.body.OnRelatedEvent(ev) {
			// Register this event as a related event.
			p.mrelated[ev.RoomInfo().ID] = relatesToID
			return
		}
		// Treat as a new message.
	}

	// Ensure that there isn't already a message with the same ID, which might
	// happen if this is a message that we sent.
	if existing, ok := p.messages[key]; ok {
		existing.ev = ev
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

	// Show the message bar if we haven't received an existing message. We put
	// this here so it doesn't get triggered if an existing message is found,
	// which usually happens if the new message is the user's.
	p.moreMsgBar.Invalidate()
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

	// Don't recreate if custom is true, since we'll override the widget that we
	// intentionally want to be different.
	recreate := !msg.custom && (false ||
		// body is nil if we've never initialized this.
		msg.body == nil ||
		// before's event ID is different for the same reason above OR we've
		// inserted a new message before this one.
		before.ev.RoomInfo().ID != msg.before ||
		// ev doesn't match up if the initialized event widget holds a different
		// event.
		!eventEq(msg.ev, msg.body.Event()))

	// Recreate the body if the raw events don't match.
	if recreate {
		msg.body = message.NewCozyMessage(p.parent.ctx, p, msg.ev, before.body)
		msg.before = ""

		if before.ev != nil {
			beforeInfo := before.ev.RoomInfo()
			msg.before = beforeInfo.ID
		}

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
func (p *Page) Load() {
	if p.loaded {
		return
	}
	p.loaded = true

	p.ctx.Renew()
	ctx := p.ctx.Take()
	client := p.parent.client.WithContext(ctx)

	fetchName := p.name == ""

	load := func(events []event.RoomEvent) {
		p.main.SetChild(p.box)
		p.list.GrabFocus()
		p.scroll.ScrollToBottom()

		// Delay adding messages a slight bit. This code allows loading all
		// messages while the fading animation is still playing, allowing some
		// breathing room for the main loop.
		const thres = 10
		const delay = 50 // total 250ms for 50 messages

		for i := 0; i < len(events); i += thres {
			i := i
			time := uint(i/thres) * delay
			load := func() {
				for j := i; j < i+thres && j < len(events); j++ {
					k := p.onRoomEvent(events[j])

					r, ok := p.messages[k]
					if ok {
						r.body.LoadMore()
					}
				}
			}

			if time == 0 {
				load()
			} else {
				glib.TimeoutAddPriority(time, glib.PriorityHighIdle, load)
			}
		}
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
			keys := make([]messageKey, len(events))
			// Require old messages first, so cozy mode works properly.
			for i, ev := range events {
				keys[i] = p.onRoomEvent(ev)
			}

			p.list.GrabFocus()

			// Load the newest messages first so it doesn't screw up scrolling as
			// hard.
			for i := len(keys) - 1; i >= 0; i-- {
				r, ok := p.messages[keys[i]]
				if ok {
					r.body.LoadMore()
				}
			}

			// Scroll to the middle message.
			for i := len(keys) - 1; i >= 0; i-- {
				r, ok := p.messages[keys[i]]
				if ok {
					glib.IdleAdd(func() { r.row.GrabFocus() })
					break
				}
			}

			// TODO: check for hasMore.
			done(true, nil)
		}
	})
}

// ScrollTo implements message.MessageViewer.
func (p *Page) ScrollTo(eventID matrix.EventID) bool {
	m, ok := p.relatedEvent(eventID)
	if ok {
		return m.row.GrabFocus()
	}
	return false
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
	field *matrix.EventID, set func(matrix.EventID) bool, class string) {

	if *field != "" {
		r, ok := p.messages[messageKeyEventID(*field)]
		if ok {
			r.row.RemoveCSSClass(class)
		}
		*field = ""
	}

	mr, ok := p.relatedEvent(eventID)
	if !ok {
		set("")
		return
	}

	if !set(eventID) {
		return
	}

	mr.row.AddCSSClass(class)
	*field = eventID
}
