package message

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/dialogs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/md/hl"
	"github.com/diamondburned/gotrix/event"
	"github.com/pkg/errors"
)

func bindParent(
	v messageViewer,
	parent gtk.Widgetter, extras ...extraMenuSetter) []gtkutil.PopoverMenuItem {

	reactor := reactor{
		ctx: v.Context,
		ev:  v.event,
	}

	roomEv := v.event.RoomInfo()

	actions := map[string]func(){
		"message.show-source": func() { showMsgSource(v.Context, v.event) },
		"message.reply":       func() { v.MessageViewer.ReplyTo(roomEv.ID) },
		"message.react":       func() { reactor.showEmoji(parent) },
		"message.react-text":  func() { reactor.showEntry(parent) },
	}

	client := v.client()

	isSelf := client.UserID == roomEv.Sender
	if isSelf {
		actions["message.edit"] = func() { v.MessageViewer.Edit(roomEv.ID) }
	}

	canRedact := isSelf || client.HasPower(roomEv.RoomID, gotktrix.RedactAction)
	if canRedact {
		actions["message.delete"] = func() { redactMessage(v) }
	}

	menuItems := []gtkutil.PopoverMenuItem{
		gtkutil.MenuItem(locale.S(v, "_Edit"), "message.edit", isSelf),
		gtkutil.MenuItem(locale.S(v, "_Reply"), "message.reply"),
		gtkutil.MenuItem(locale.S(v, "Add Rea_ction"), "message.react"),
		gtkutil.MenuItem(locale.S(v, "Add Reaction with _Text"), "message.react-text"),
		gtkutil.MenuItem(locale.S(v, "_Delete"), "message.delete", canRedact),
		gtkutil.MenuItem(locale.S(v, "Show _Source"), "message.show-source"),
	}

	gtkutil.BindActionMap(parent, actions)
	gtkutil.BindPopoverMenuCustom(parent, gtk.PosBottom, menuItems)

	extraItems := gtkutil.CustomMenu(menuItems)
	for _, extra := range extras {
		extra.SetExtraMenu(extraItems)
	}

	return menuItems
}

type extraMenuSetter interface {
	SetExtraMenu(gio.MenuModeller)
}

var (
	_ extraMenuSetter = (*gtk.Label)(nil)
	_ extraMenuSetter = (*gtk.TextView)(nil)
)

func redactMessage(v messageViewer) {
	// TODO: confirmation menu
	client := v.client()
	roomEv := v.event.RoomInfo()

	if err := client.Redact(roomEv.RoomID, roomEv.ID, ""); err != nil {
		app.Error(v, errors.Wrap(err, "cannot delete message"))
	}
}

var reactCSS = cssutil.Applier("message-react", `
	entry.message-react {
		margin: 6px;
	}
`)

type reactor struct {
	ctx context.Context
	ev  event.RoomEvent
}

func (r *reactor) showEmoji(parent gtk.Widgetter) *gtk.EmojiChooser {
	picker := gtk.NewEmojiChooser()
	picker.SetParent(parent)
	picker.SetPosition(gtk.PosBottom)
	picker.SetAutohide(true)
	picker.ConnectEmojiPicked(func(emoji string) {
		r.react(emoji)
		picker.Popdown()
	})
	reactCSS(&picker.Widget)
	gtkutil.PopupFinally(picker)
	return picker
}

func (r *reactor) showEntry(parent gtk.Widgetter) *gtk.Entry {
	entry := gtk.NewEntry()
	entry.SetHExpand(true)
	entry.SetVAlign(gtk.AlignCenter)
	entry.SetObjectProperty("show-emoji-icon", true)
	entry.SetObjectProperty("enable-emoji-completion", true)
	entry.SetPlaceholderText("¯\\_(ツ)_/¯")
	reactCSS(entry)

	b := adaptive.NewBin()
	b.SetChild(entry)

	d := dialogs.NewLocalize(r.ctx, "Cancel", "React")
	d.SetTitle("React to Message")
	d.SetDefaultSize(-1, -1)
	d.SetChild(b)
	d.Show()

	d.Cancel.ConnectClicked(d.Close)

	submit := func() {
		text := entry.Text()
		if text != "" {
			r.react(text)
			entry.SetText("")
		}
		d.Close()
	}

	d.OK.ConnectClicked(submit)
	entry.ConnectActivate(submit)

	return entry
}

func (r *reactor) react(text string) {
	roomEv := r.ev.RoomInfo()

	ev := m.ReactionEvent{
		RoomEventInfo: event.RoomEventInfo{
			EventInfo: event.EventInfo{
				Type: m.ReactionEventType,
			},
			RoomID: roomEv.RoomID,
		},
		RelatesTo: m.ReactionRelatesTo{
			RelType: m.Annotation,
			EventID: roomEv.ID,
			Key:     text,
		},
	}

	go func() {
		client := gotktrix.FromContext(r.ctx)
		if err := client.SendRoomEvent(ev.RoomID, &ev); err != nil {
			app.Error(r.ctx, errors.Wrap(err, "failed to react"))
		}
	}()
}

var sourceCSS = cssutil.Applier("message-source", `
	.message-source {
		padding: 6px 4px;
		font-family: monospace;
	}
`)

type partialRoomEvent struct {
	event.RoomEventInfo
	Content interface{} `json:"content"`
}

func showMsgSource(ctx context.Context, event event.RoomEvent) {
	raw := event.Info().Raw
	if raw != nil {
		var buf bytes.Buffer
		if json.Indent(&buf, raw, "", "  ") == nil {
			raw = buf.Bytes()
		}
	} else {
		j, err := json.MarshalIndent(partialRoomEvent{
			RoomEventInfo: *event.RoomInfo(),
			Content:       event,
		}, "", "  ")
		if err != nil {
			app.Error(ctx, err)
		}
		raw = []byte("// Event missing raw JSON.\n")
		raw = append(raw, j...)
	}

	d := gtk.NewDialog()
	d.SetTransientFor(app.GTKWindowFromContext(ctx))
	d.SetModal(true)
	d.SetDefaultSize(400, 300)

	buf := gtk.NewTextBuffer(nil)
	buf.SetText(string(raw))
	hl.Highlight(ctx, buf.StartIter(), buf.EndIter(), "json")

	t := gtk.NewTextViewWithBuffer(buf)
	t.SetEditable(false)
	t.SetCursorVisible(false)
	t.SetWrapMode(gtk.WrapWordChar)
	sourceCSS(t)

	s := gtk.NewScrolledWindow()
	s.SetVExpand(true)
	s.SetHExpand(true)
	s.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	s.SetChild(t)

	box := d.ContentArea()
	box.Append(s)

	d.Show()
}
