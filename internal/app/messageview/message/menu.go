package message

import (
	"context"
	"encoding/json"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
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

	actions := map[string]func(){
		"show-source": func() {
			showMsgSource(app.FromContext(v.Context).Window(), v.raw.RawEvent)
		},
		"reply": func() {
			v.MessageViewer.ReplyTo(v.raw.ID)
		},
		"react": nil,
	}

	uID, _ := v.client().Whoami()
	canEdit := uID == v.raw.Sender

	if canEdit {
		actions["edit"] = func() {
			v.MessageViewer.Edit(v.raw.ID)
		}
	}

	messageMenuItems := []gtkutil.PopoverMenuItem{
		gtkutil.MenuItem("Edit", "message.edit", canEdit),
		gtkutil.MenuItem("Reply", "message.reply"),
		gtkutil.Submenu("React", []gtkutil.PopoverMenuItem{
			gtkutil.MenuWidget("message.react", reactEntry(v.Context, v.raw.RawEvent)),
		}),
		gtkutil.MenuItem("Show Source", "message.show-source"),
	}

	gtkutil.BindActionMap(parent, "message", actions)
	gtkutil.BindPopoverMenuCustom(parent, gtk.PosBottom, messageMenuItems)

	for _, extra := range extras {
		bindExtraMenu(extra, messageMenuItems)
	}

	return messageMenuItems
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
	.message-react {
		margin: 0 6px;
	}
`)

func reactEntry(ctx context.Context, targ *event.RawEvent) gtk.Widgetter {
	entry := gtk.NewEntry()
	entry.SetHExpand(true)
	entry.SetObjectProperty("show-emoji-icon", true)
	entry.SetObjectProperty("enable-emoji-completion", true)
	entry.SetPlaceholderText("Add Reaction")
	entry.Connect("activate", func() {
		text := entry.Text()
		if text == "" {
			return
		}

		entry.SetText("")

		ev := m.ReactionEvent{
			RoomEventInfo: event.RoomEventInfo{
				RoomID: targ.RoomID,
			},
			RelatesTo: m.ReactionRelatesTo{
				RelType: m.Annotation,
				EventID: targ.ID,
				Key:     text,
			},
		}

		go func() {
			client := gotktrix.FromContext(ctx)
			if err := client.SendRoomEvent(ev.RoomID, ev); err != nil {
				app.Error(ctx, errors.Wrap(err, "failed to react"))
			}
		}()
	})
	reactCSS(entry)

	return entry
}

var msgSourceAttrs = markuputil.Attrs(
	pango.NewAttrFamily("monospace"),
)

var sourceCSS = cssutil.Applier("message-source", `
	.message-source {
		padding: 6px 4px;
	}
`)

func showMsgSource(w *gtk.Window, raw *event.RawEvent) {
	j, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		errpopup.Show(w, []error{err}, func() {})
		return
	}

	d := gtk.NewDialog()
	d.SetTransientFor(w)
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
