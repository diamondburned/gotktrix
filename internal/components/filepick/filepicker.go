package filepick

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
)

var useNative = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Native File Picker",
	Section:     "Application",
	Description: "Use the system native file picker instead of GTK's.",
})

type dialog interface {
	Show()
	ConnectResponse(func(int)) glib.SignalHandle
}

// FilePicker is the file chooser.
type FilePicker struct {
	*gtk.FileChooser
	dialog dialog
}

// New creates a new file chooser.
func New(ctx context.Context, title string, action gtk.FileChooserAction, accept, cancel string) *FilePicker {
	return NewWithWindow(app.Window(ctx), title, action, accept, cancel)
}

// NewWithWindow creates a file chooser with the given parent window.
func NewWithWindow(parent *gtk.Window, title string, action gtk.FileChooserAction, accept, cancel string) *FilePicker {
	var p FilePicker

	if useNative.Value() {
		native := gtk.NewFileChooserNative(title, parent, action, accept, cancel)

		p.FileChooser = &native.FileChooser
		p.dialog = native
	} else {
		w := gtk.NewFileChooserWidget(action)

		dialog := gtk.NewDialogWithFlags(
			title, parent,
			gtk.DialogUseHeaderBar|gtk.DialogModal|gtk.DialogDestroyWithParent)
		dialog.SetDefaultSize(750, 550)
		dialog.SetChild(w)
		dialog.Connect("response", dialog.Destroy)

		p.FileChooser = &w.FileChooser
		p.dialog = dialog

		acceptButton := dialog.AddButton(accept, int(gtk.ResponseAccept)).(*gtk.Button)
		acceptButton.AddCSSClass("suggested-action")

		dialog.AddButton(cancel, int(gtk.ResponseCancel))

		header := dialog.HeaderBar()
		header.SetShowTitleButtons(false)
	}

	return &p
}

// Show shows the dialog.
func (p *FilePicker) Show() {
	p.dialog.Show()
}

// ConnectResponse connects to the dialog's response signal.
func (p *FilePicker) ConnectResponse(f func(gtk.ResponseType)) {
	p.dialog.ConnectResponse(func(respID int) {
		f(gtk.ResponseType(respID))
	})
}

// ConnectAccept connects to the dialog's Accept response.
func (p *FilePicker) ConnectAccept(f func()) {
	p.ConnectResponse(func(resp gtk.ResponseType) {
		if resp == gtk.ResponseAccept {
			f()
		}
	})
}
