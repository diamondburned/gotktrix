package errpopup

import (
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

var css = cssutil.Applier("errpopup", `
	.errpopup label.error {
		margin: .5em .75em;
	}
`)

// Fatal shows a popup that closes the window once it's closed.
func Fatal(parent *gtk.Window, errors ...error) {
	Show(parent, errors, parent.Close)
	parent.SetSensitive(false)
}

// Show shows a popup with the given errors.
func Show(parent *gtk.Window, errors []error, done func()) {
	glib.IdleAdd(func() { show(parent, errors, done) })
}

type dialogState struct {
	next   *gtk.Button
	errors []string
}

var windows = make(map[*gtk.Window]*dialogState)

func show(parent *gtk.Window, errors []error, done func()) {
	if len(errors) == 0 {
		done()
		return
	}

	errStrings := make([]string, len(errors))
	for i := range errStrings {
		errStrings[i] = markuputil.IndentError(errors[i].Error())
	}

	state, ok := windows[parent]
	if ok {
		state.errors = append(state.errors, errStrings...)
		state.next.SetLabel("Next")
		return
	}

	dialog := gtk.NewDialog()
	dialog.SetDefaultSize(400, 200)
	dialog.SetTitle("Error")
	dialog.SetTransientFor(parent)
	dialog.SetModal(true)

	errorStack := gtk.NewStack()
	errorStack.SetVAlign(gtk.AlignStart)
	errorStack.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)

	scroll := gtk.NewScrolledWindow()
	scroll.SetHExpand(true)
	scroll.SetChild(errorStack)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)

	content := dialog.ContentArea()
	content.SetVExpand(true)
	content.Append(scroll)
	css(content)

	nextButton := dialog.AddButton("Next", int(gtk.ResponseOK)).(*gtk.Button)
	dialog.SetDefaultWidget(nextButton)

	state = &dialogState{
		next:   nextButton,
		errors: errStrings,
	}

	windows[parent] = state

	var ix int

	cycle := func() {
		if ix == len(state.errors) {
			dialog.Close()
			done()

			delete(windows, parent)
			return
		}

		if ix == len(state.errors)-1 {
			// Showing last one.
			nextButton.SetLabel("OK")
		}

		// Set error.
		label := markuputil.ErrorLabel(state.errors[ix])
		errorStack.AddChild(label)
		errorStack.SetVisibleChild(label)

		// Scroll up.
		vadj := scroll.VAdjustment()
		vadj.SetValue(0)

		ix++
	}
	// Show the first error.
	cycle()

	dialog.Connect("response", cycle)
	dialog.Show()
}
