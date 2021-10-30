package dialogs

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

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

	return &Dialog{
		Dialog: dialog,
		OK:     okBtn,
		Cancel: ccBtn,
	}
}
