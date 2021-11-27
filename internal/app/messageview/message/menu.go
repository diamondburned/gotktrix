package message

import (
	"context"
	"encoding/json"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/components/dialogs"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/pkg/errors"
)

func bindParent(
	v messageViewer,
	parent gtk.Widgetter, extras ...extraMenuSetter) []gtkutil.PopoverMenuItem {

	reactor := reactor{
		ctx: v.Context,
		raw: v.raw.RawEvent,
	}

	actions := map[string]func(){
		"show-source": func() {
			showMsgSource(v.Context, v.raw.RawEvent)
		},
		"reply": func() {
			v.MessageViewer.ReplyTo(v.raw.ID)
		},
		"react": func() {
			showReactEmoji(&reactor, parent)
		},
		"react-text": func() {
			showReactEntry(&reactor, parent)
		},
	}

	uID, _ := v.client().Whoami()
	canEdit := uID == v.raw.Sender

	if canEdit {
		actions["edit"] = func() {
			v.MessageViewer.Edit(v.raw.ID)
		}
	}

	menuItems := []gtkutil.PopoverMenuItem{
		gtkutil.MenuItem("Edit", "message.edit", canEdit),
		gtkutil.MenuItem("Reply", "message.reply"),
		gtkutil.MenuItem("Add Reaction", "message.react"),
		gtkutil.MenuItem("Add Reaction with Text", "message.react-text"),
		gtkutil.MenuItem("Show Source", "message.show-source"),
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
	raw *event.RawEvent
}

func (r *reactor) react(text string) {
	ev := m.ReactionEvent{
		RoomEventInfo: event.RoomEventInfo{
			RoomID: r.raw.RoomID,
		},
		RelatesTo: m.ReactionRelatesTo{
			RelType: m.Annotation,
			EventID: r.raw.ID,
			Key:     text,
		},
	}

	go func() {
		client := gotktrix.FromContext(r.ctx)
		if err := client.SendRoomEvent(ev.RoomID, ev); err != nil {
			app.Error(r.ctx, errors.Wrap(err, "failed to react"))
		}
	}()
}

var msgSourceAttrs = markuputil.Attrs(
	pango.NewAttrFamily("monospace"),
)

var sourceCSS = cssutil.Applier("message-source", `
	.message-source {
		padding: 6px 4px;
	}
`)

func showMsgSource(ctx context.Context, raw *event.RawEvent) {
	j, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		app.Error(ctx, err)
		return
	}

	d := gtk.NewDialog()
	d.SetTransientFor(app.Window(ctx))
	d.SetModal(true)
	d.SetDefaultSize(400, 300)

	l := gtk.NewLabel(string(j))
	l.SetSelectable(true)
	l.SetAttributes(msgSourceAttrs)
	l.SetWrap(true)
	l.SetWrapMode(pango.WrapWordChar)
	l.SetXAlign(0)
	sourceCSS(l)

	s := gtk.NewScrolledWindow()
	s.SetVExpand(true)
	s.SetHExpand(true)
	s.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	s.SetChild(l)

	box := d.ContentArea()
	box.Append(s)

	d.Show()
}
