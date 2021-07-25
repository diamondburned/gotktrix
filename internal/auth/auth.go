// Package auth supplies a gtk.Assistant wrapper to provide a login screen.
package auth

import (
	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/api/httputil"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

var keyringAppID = config.AppIDDot("secrets")

type Assistant struct {
	*assistant.Assistant
	client httputil.Client

	onConnect func(*gotrix.Client)

	// states, can be nil depending on the steps
	accounts      []account
	currentClient *gotrix.Client

	// hasConnected is true if the connection has already been connected.
	hasConnected bool
}

type discoverStep struct {
	// states
	serverName string
}

// New creates a new authentication assistant with the default HTTP client.
func New(parent *gtk.Window) *Assistant {
	return NewWithClient(parent, httputil.NewClient())
}

// NewWithClient creates a new authentication assistant with the given HTTP
// client.
func NewWithClient(parent *gtk.Window, client httputil.Client) *Assistant {
	ass := assistant.New(parent, nil)
	ass.SetTitle("Getting Started")

	a := Assistant{
		Assistant: ass,
		client:    client,
	}

	ass.Connect("close", func() {
		// If the user hasn't chosen to connect to anything yet, then exit the
		// main window as well.
		if !a.hasConnected {
			parent.Close()
		}
	})
	ass.AddStep(accountChooserStep(&a))
	return &a
}

// OnConnect sets the handler that is called when the user chooses an account or
// logs in. If this method has already been called before with a non-nil
// function, it will panic.
func (a *Assistant) OnConnect(f func(*gotrix.Client)) {
	if a.onConnect != nil {
		panic("OnConnect called twice")
	}

	a.onConnect = f
}

func (a *Assistant) signinPage() {
	step2 := homeserverStep(a)
	a.AddStep(step2)
	a.SetStep(step2)

	step3 := loginStep(a)
	a.AddStep(step3)
}

func (a *Assistant) chooseHomeserver(client *gotrix.Client) {
	a.currentClient = client
}

func (a *Assistant) chooseAccount(acc account) {}

var inputBoxCSS = cssutil.Applier("auth-input-box", `
	.auth-input-box {
		margin-top: 4px;
	}
	.auth-input-box label {
		margin-left: .5em;
	}
	.auth-input-box > entry {
		margin-bottom: 4px;
	}
`)

var inputLabelAttrs = markuputil.Attrs(
	pango.NewAttrForegroundAlpha(65535 * 90 / 100), // 90%
)

func makeInputs(names ...string) (gtk.Widgetter, []*gtk.Entry) {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetSizeRequest(200, -1)
	inputBoxCSS(box)

	entries := make([]*gtk.Entry, len(names))

	for i, name := range names {
		label := gtk.NewLabel(name)
		label.SetXAlign(0)
		label.SetAttributes(inputLabelAttrs)

		entry := gtk.NewEntry()
		entry.SetEnableUndo(true)

		box.Append(label)
		box.Append(entry)

		entries[i] = entry
	}

	return box, entries
}
