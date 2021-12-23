package message

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/components/dialogs"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/md/hl"
	"github.com/pkg/errors"
)

func bindParent(
	v messageViewer,
	parent gtk.Widgetter, extras ...extraMenuSetter) []gtkutil.PopoverMenuItem {

	reactor := reactor{
		ctx: v.Context,
		ev:  v.event,
	}

	actions := map[string]func(){
		"show-source": func() {
			showMsgSource(v.Context, v.event)
		},
		"reply": func() {
			v.MessageViewer.ReplyTo(v.event.RoomInfo().ID)
		},
		"react": func() {
			showReactEmoji(&reactor, parent)
		},
		"react-text": func() {
			showReactEntry(&reactor, parent)
		},
	}

	uID, _ := v.client().Whoami()
	canEdit := uID == v.event.RoomInfo().Sender

	if canEdit {
		actions["edit"] = func() {
			v.MessageViewer.Edit(v.event.RoomInfo().ID)
		}
	}

	menuItems := []gtkutil.PopoverMenuItem{
		gtkutil.MenuItem("_Edit", "message.edit", canEdit),
		gtkutil.MenuItem("_Reply", "message.reply"),
		gtkutil.MenuItem("Add Rea_ction", "message.react"),
		gtkutil.MenuItem("Add Reaction with _Text", "message.react-text"),
		gtkutil.MenuItem("Show _Source", "message.show-source"),
	}

	gtkutil.BindActionMap(parent, "message", actions)
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

func bindExtraMenu(m extraMenuSetter, items []gtkutil.PopoverMenuItem) {
	m.SetExtraMenu(gtkutil.CustomMenu(items))
}

var reactCSS = cssutil.Applier("message-react", `
	entry.message-react {
		margin: 6px;
	}
`)

func showReactEmoji(r *reactor, parent gtk.Widgetter) *gtk.EmojiChooser {
	picker := gtk.NewEmojiChooser()
	picker.SetParent(parent)
	picker.SetPosition(gtk.PosBottom)
	picker.SetAutohide(true)
	picker.Connect("emoji-picked", func(emoji string) {
		r.react(emoji)
		picker.Popdown()
	})
	reactCSS(&picker.Widget)
	picker.Popup()
	return picker
}

func showReactEntry(r *reactor, parent gtk.Widgetter) *gtk.Entry {
	entry := gtk.NewEntry()
	entry.SetHExpand(true)
	entry.SetVAlign(gtk.AlignCenter)
	entry.SetObjectProperty("show-emoji-icon", true)
	entry.SetObjectProperty("enable-emoji-completion", true)
	entry.SetPlaceholderText("¯\\_(ツ)_/¯")
	reactCSS(entry)

	b := adaptive.NewBin()
	b.SetChild(entry)

	d := dialogs.New(app.Window(r.ctx), "Cancel", "React")
	d.SetTitle("React to Message")
	d.SetDefaultSize(-1, -1)
	d.SetChild(b)
	d.Show()

	d.Cancel.Connect("clicked", d.Close)

	submit := func() {
		text := entry.Text()
		if text != "" {
			r.react(text)
			entry.SetText("")
		}
		d.Close()
	}

	d.OK.Connect("clicked", submit)
	entry.Connect("activate", submit)

	return entry
}

type reactor struct {
	ctx context.Context
	ev  event.RoomEvent
}

func (r *reactor) react(text string) {
	roomEv := r.ev.RoomInfo()

	ev := m.ReactionEvent{
		RoomEventInfo: event.RoomEventInfo{
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
		if err := client.SendRoomEvent(ev.RoomID, r.ev); err != nil {
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
	d.SetTransientFor(app.Window(ctx))
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
