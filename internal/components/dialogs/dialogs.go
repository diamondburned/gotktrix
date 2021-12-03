package dialogs

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Dialog provides a dialog with a headerbar.
type Dialog struct {
	*gtk.Dialog

	OK     *gtk.Button
	Cancel *gtk.Button
}

// New creates a new dialog.
func New(transientFor *gtk.Window, cancel, ok string) *Dialog {
	const dialogFlags = 0 |
		gtk.DialogDestroyWithParent |
		gtk.DialogModal |
		gtk.DialogUseHeaderBar

	dialog := gtk.NewDialogWithFlags("", transientFor, dialogFlags)
	dialog.SetDefaultSize(375, 500)

	okBtn := dialog.AddButton(ok, int(gtk.ResponseOK)).(*gtk.Button)
	okBtn.AddCSSClass("suggested-action")
	ccBtn := dialog.AddButton(cancel, int(gtk.ResponseCancel)).(*gtk.Button)

	esc := gtk.NewEventControllerKey()
	esc.SetName("dialog-escape")
	esc.ConnectKeyPressed(func(val, code uint, state gdk.ModifierType) bool {
		switch val {
		case gdk.KEY_Escape:
			if ccBtn.Sensitive() {
				ccBtn.Activate()
				return true
			}
		}

		return false
	})
	dialog.AddController(esc)

	return &Dialog{
		Dialog: dialog,
		OK:     okBtn,
		Cancel: ccBtn,
	}
}

// BindEnterOK binds the Enter key to activate the OK button.
func (d *Dialog) BindEnterOK() {
	ev := gtk.NewEventControllerKey()
	ev.SetName("dialog-ok")
	ev.ConnectKeyPressed(func(val, code uint, state gdk.ModifierType) bool {
		switch val {
		case gdk.KEY_Return:
			if d.OK.Sensitive() {
				d.OK.Activate()
				return true
			}
		}

		return false
	})
	d.Dialog.AddController(ev)
}

// BindCancelClose binds cancel to closing the dialog.
func (d *Dialog) BindCancelClose() {
	d.Cancel.ConnectClicked(func() {
		d.Dialog.Close()
		d.Dialog.Destroy()
	})
}
