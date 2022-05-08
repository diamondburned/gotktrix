// Package auth supplies a gtk.Assistant wrapper to provide a login screen.
package auth

import (
	"context"
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/secret"
	"github.com/diamondburned/gotrix/api/httputil"
	"github.com/diamondburned/gotrix/matrix"
)

type Assistant struct {
	*assistant.Assistant
	ctx    context.Context
	client httputil.Client

	onConnect func(*gotktrix.Client, *Account)

	// states, can be nil depending on the steps
	accounts      []assistantAccount
	currentClient *gotktrix.ClientAuth

	keyring     *secret.Keyring
	encrypt     *secret.EncryptedFile
	encryptPath string

	// hasConnected is true if the connection has already been connected.
	hasConnected bool
}

type assistantAccount struct {
	*Account
	src secret.Driver
}

// Show creates a new authentication assistant with the default HTTP client.
func Show(ctx context.Context) *Assistant {
	return ShowWithClient(ctx, httputil.NewClient())
}

// ShowWithClient creates a new authentication assistant with the given HTTP
// client.
func ShowWithClient(ctx context.Context, client httputil.Client) *Assistant {
	ass := assistant.Use(app.GTKWindowFromContext(ctx), nil)
	ass.SetTitle("Getting Started")

	app := app.FromContext(ctx)

	a := Assistant{
		Assistant:   ass,
		ctx:         ctx,
		client:      client,
		keyring:     secret.KeyringDriver(app.IDDot("secrets")),
		encryptPath: app.ConfigPath("secrets"),
	}

	ass.AddStep(accountChooserStep(&a))
	ass.Show()
	return &a
}

// OnConnect sets the handler that is called when the user chooses an account or
// logs in. If this method has already been called before with a non-nil
// function, it will panic.
func (a *Assistant) OnConnect(f func(*gotktrix.Client, *Account)) {
	if a.onConnect != nil {
		panic("OnConnect called twice")
	}

	a.onConnect = f
}

// step 1 activate
func (a *Assistant) signinPage() {
	step2 := homeserverStep(a)
	a.AddStep(step2)
	a.SetStep(step2)
}

// step 2 activate
func (a *Assistant) chooseHomeserver(client *gotktrix.ClientAuth, methods []matrix.LoginMethod) {
	a.currentClient = client

	step3 := chooseLoginStep(a, methods)
	a.AddStep(step3)
	a.SetStep(step3)
}

// step 3 activate
func (a *Assistant) chooseLoginMethod(method matrix.LoginMethod) {
	step4 := loginStep(a, method)
	a.AddStep(step4)
	a.SetStep(step4)
}

// finish should be called once a.currentClient has been logged on.
func (a *Assistant) finish(c *gotktrix.Client, acc *Account) {
	if a.onConnect == nil {
		log.Println("onConnect handler not attached")
		return
	}

	// Restore context.
	c = c.WithContext(a.ctx)

	a.hasConnected = true
	a.Continue()
	a.onConnect(c, acc)
}

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

var inputLabelAttrs = textutil.Attrs(
	pango.NewAttrForegroundAlpha(65535 * 90 / 100), // 90%
)

func (a *Assistant) makeInputs(names ...string) (gtk.Widgetter, []*gtk.Entry) {
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

		if i < len(names)-1 {
			// Enter moves to the next entry.
			next := i + 1
			entry.ConnectActivate(func() { entries[next].GrabFocus() })
		} else {
			// Enter hits the OK button.
			entry.ConnectActivate(func() { a.OKButton().Activate() })
		}

		box.Append(label)
		box.Append(entry)

		entries[i] = entry
	}

	return box, entries
}

var errorLabelCSS = cssutil.Applier("auth-error-label", `
	.auth-error-label {
		padding-top: 4px;
	}
`)

func makeErrorLabel() *gtk.Label {
	errLabel := textutil.ErrorLabel("")
	errorLabelCSS(errLabel)
	return errLabel
}
