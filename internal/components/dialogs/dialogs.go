package dialogs

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// Dialog provides a dialog with a headerbar.
type Dialog struct {
	*gtk.Window
	Header *gtk.HeaderBar

	OK     *gtk.Button
	Cancel *gtk.Button
}

// New creates a new dialog.
func New(transientFor *gtk.Window, cancel, ok string) *Dialog {
	okButton := gtk.NewButtonWithLabel(ok)
	okButton.AddCSSClass("suggested-action")

	cancelButton := gtk.NewButtonWithLabel(cancel)

	bar := gtk.NewHeaderBar()
	bar.SetShowTitleButtons(false)
	bar.PackStart(cancelButton)
	bar.PackEnd(okButton)

	window := gtk.NewWindow()
	window.SetDefaultSize(375, 500)
	window.SetTransientFor(transientFor)
	window.SetModal(true)
	window.SetTitlebar(bar)

	return &Dialog{
		Window: window,
		Header: bar,
		OK:     okButton,
		Cancel: cancelButton,
	}
}
