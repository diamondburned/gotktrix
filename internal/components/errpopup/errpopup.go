package errpopup

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/gotk3/gotk3/glib"
)

var css = cssutil.Applier("errpopup", `
	.errpopup label.error {
		margin: .5em .75em;
	}
`)

// ShowFatal shows a popup that closes the window once it's closed.
func ShowFatal(parent *gtk.Window, errors []error) {
	Show(parent, errors, parent.Close)
	parent.SetSensitive(false)
}

// Show shows a popup with the given errors.
func Show(parent *gtk.Window, errors []error, done func()) {
	glib.IdleAdd(func() { show(parent, errors, done) })
}

func show(parent *gtk.Window, errors []error, done func()) {
	if len(errors) == 0 {
		done()
		return
	}

	strings := make([]string, len(errors))
	for i := range strings {
		strings[i] = markuputil.Error(errors[i].Error())
	}

	dialog := gtk.NewDialog()
	dialog.SetTitle("Error")
	dialog.SetTransientFor(parent)
	dialog.SetModal(true)

	errorStack := gtk.NewStack()
	errorStack.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)

	content := dialog.ContentArea()
	content.SetVExpand(true)
	content.SetVAlign(gtk.AlignStart)
	content.Append(errorStack)
	css(content)

	errLabels := make([]*gtk.Label, len(errors))
	for i := range errors {
		errLabels[i] = markuputil.ErrorLabel(markuputil.Error(errors[i].Error()))
		errorStack.AddChild(errLabels[i])
	}

	nextButton := dialog.AddButton("Next", int(gtk.ResponseOk)).(*gtk.Button)
	dialog.SetDefaultWidget(nextButton)

	var ix int

	cycle := func() {
		if ix == len(errors) {
			dialog.Close()
			done()
			return
		}

		if ix == len(errors)-1 {
			// Showing last one.
			nextButton.SetLabel("OK")
		}

		errorStack.SetVisibleChild(errLabels[ix])
		ix++
	}
	// Show the first error.
	cycle()

	dialog.Connect("response", cycle)
	dialog.Show()
}
