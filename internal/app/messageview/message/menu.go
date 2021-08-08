package message

import (
	"encoding/json"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

var messageMenuItems = [][2]string{
	{"Show Source", "message.show-source"},
}

func bind(p MessageViewer, m Message) {
	actions := map[string]func(){
		"show-source": func() { showMsgSource(p.Window(), m.Event()) },
	}

	gtkutil.BindActionMap(m, "message", actions)
	gtkutil.BindPopoverMenu(m, gtk.PosBottom, messageMenuItems)
}

type extraMenuSetter interface {
	SetExtraMenu(gio.MenuModeller)
}

var (
	_ extraMenuSetter = (*gtk.Label)(nil)
	_ extraMenuSetter = (*gtk.TextView)(nil)
)

func bindExtraMenu(m extraMenuSetter) {
	m.SetExtraMenu(gtkutil.MenuPair(messageMenuItems))
}

var msgSourceAttrs = markuputil.Attrs(
	pango.NewAttrFamily("monospace"),
)

var sourceCSS = cssutil.Applier("message-source", `
	.message-source {
		padding: 6px 4px;
	}
`)

func showMsgSource(w *gtk.Window, ev event.Event) {
	j, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		errpopup.Show(w, []error{err}, func() {})
		return
	}

	d := gtk.NewDialog()
	d.SetTransientFor(w)
	d.SetModal(true)
	d.SetDefaultSize(400, 300)

	l := gtk.NewLabel(string(j))
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

// func rawEvent(ev event.Event) *event.RawEvent {
// 	raw := event.RawEvent{
// 		Type: ev.Type(),
// 	}

// 	if room, ok := ev.(event.RoomEvent); ok {
// 		raw.ID = room.ID()
// 		raw.RoomID = room.Room()
// 		raw.Sender = room.Sender()
// 		raw.OriginServerTime = room.OriginServerTime()
// 	}

// 	if state, ok := ev.(event.StateEvent); ok {
// 		raw.StateKey = state.StateKey()
// 	}

// 	j, err := json.Marshal()
// }
