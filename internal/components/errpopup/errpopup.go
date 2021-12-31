package errpopup

import (
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

var dialogCSS = cssutil.Applier("errpopup", `
	.errpopup .title,
	.errpopup button {
		color: @error_color;
	}
`)

// Fatal shows a popup that closes the window once it's closed.
func Fatal(parent *gtk.Window, errors ...error) {
	Show(parent, errors, parent.Close)
	parent.SetSensitive(false)
}

// Show shows a popup with the given errors.
func Show(parent *gtk.Window, errors []error, done func()) {
	gtkutil.InvokeMain(func() { show(parent, errors, done) })
}

type dialogState struct {
	parent *gtk.Window
	dialog *gtk.MessageDialog
	done   func()

	errors []string
	ix     int
}

var windows = make(map[*gtk.Window]*dialogState)

func indentError(msg string) string {
	parts := strings.Split(msg, ": ")

	var builder strings.Builder
	builder.WriteString(parts[0])

	for i, part := range parts[1:] {
		builder.WriteByte('\n')
		builder.WriteString(strings.Repeat(" ", (i+1)*3))
		builder.WriteString("- ")
		builder.WriteString(html.EscapeString(part))
	}

	return builder.String()
}

func show(parent *gtk.Window, errors []error, done func()) {
	if len(errors) == 0 {
		done()
		return
	}

	errStrings := make([]string, len(errors))
	for i := range errStrings {
		errStrings[i] = indentError(errors[i].Error())
	}

	state, ok := windows[parent]
	if ok {
		state.errors = append(state.errors, errStrings...)
		// Chain this awful thing up.
		prevDone := state.done
		state.done = func() {
			prevDone()
			done()
		}
		return
	}

	dialog := gtk.NewMessageDialog(
		parent,
		gtk.DialogModal|gtk.DialogDestroyWithParent|gtk.DialogUseHeaderBar,
		gtk.MessageError, gtk.ButtonsOK)
	dialog.SetObjectProperty("secondary-use-markup", true)
	dialogCSS(dialog)

	state = &dialogState{
		parent: parent,
		dialog: dialog,
		done:   done,
		errors: errStrings,
	}

	windows[parent] = state

	state.cycle()
	dialog.Connect("response", state.cycle)

	dialog.Show()
}

func (s *dialogState) cycle() {
	if s.ix == len(s.errors) {
		s.dialog.Destroy()
		s.done()
		delete(windows, s.parent)
		return
	}

	s.dialog.SetMarkup(fmt.Sprintf(
		`Error <span size="xx-small">(%d/%d)</span>`,
		s.ix+1, len(s.errors),
	))
	s.dialog.SetObjectProperty("secondary-text", s.errors[s.ix])
	s.ix++
}
