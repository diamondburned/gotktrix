package auth

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/secret"
	"github.com/gotk3/gotk3/glib"
)

type loginStepData struct {
	InputBox gtk.Widgetter
	Login    func(client *gotktrix.Client) error
}

func loginStep(a *Assistant, method loginMethod) *assistant.Step {
	var data loginStepData

	switch method {
	case loginPassword:
		inputBox, inputs := a.makeInputs("Username (or Email)", "Password")
		inputs[0].SetInputPurpose(gtk.InputPurposeEmail)
		inputs[1].SetInputPurpose(gtk.InputPurposePassword)
		inputs[1].SetVisibility(false)

		data.InputBox = inputBox
		data.Login = func(client *gotktrix.Client) error {
			return client.LoginPassword(inputs[0].Text(), inputs[1].Text())
		}
	case loginToken:
		inputBox, inputs := a.makeInputs("Token")
		inputs[0].SetInputPurpose(gtk.InputPurposePassword)
		inputs[0].SetVisibility(false)

		data.InputBox = inputBox
		data.Login = func(client *gotktrix.Client) error {
			return client.LoginToken(inputs[0].Text())
		}
	}

	errLabel := makeErrorLabel()
	errLabel.Hide()

	rememberMe := newRememberMeBox(a)

	step := assistant.NewStep("Password", "Log in")
	// step.CanBack = true

	content := step.ContentArea()
	content.SetOrientation(gtk.OrientationVertical)
	content.Append(data.InputBox)
	content.Append(errLabel)
	content.Append(rememberMe)

	onError := func(err error) {
		errLabel.SetMarkup(markuputil.Error(err.Error()))
		errLabel.Show()
		a.Continue()
	}

	step.Done = func(step *assistant.Step) {
		ctx := a.CancellableBusy(context.Background())

		go func() {
			client := a.currentClient.WithContext(ctx)

			if err := data.Login(client); err != nil {
				glib.IdleAdd(func() { onError(err) })
				return
			}

			acc, err := copyAccount(client)
			if err != nil {
				glib.IdleAdd(func() { onError(err) })
				return
			}

			glib.IdleAdd(func() {
				// Assistant is still busy at this point.
				rememberMe.saveAndFinish(a, acc)
			})
		}()
	}

	return step
}

type rememberMeBox struct {
	*gtk.Box
	keyring bool
	encrypt bool

	password string // only if encrypted == false
}

var rememberMeCSS = cssutil.Applier("auth-remember-me", `
	.auth-remember-me > revealer > box > checkbutton {
		margin-left: 20px;
	}
`)

var rememberMePasswordCSS = cssutil.Applier("auth-remember-me-password", `
	.auth-remember-me-password {
		margin: 6px 0;
		margin-top: 6px;
	}
	.auth-remember-me-password label {
		margin-left: .5em;
	}
`)

func newRememberMeBox(a *Assistant) *rememberMeBox {
	var state rememberMeBox

	box := gtk.NewBox(gtk.OrientationVertical, 0)

	var useKeyring *gtk.CheckButton
	if a.keyring != nil {
		useKeyring = gtk.NewCheckButtonWithLabel("System Keyring")
		useKeyring.Connect("toggled", func(useKeyring *gtk.CheckButton) {
			state.keyring = useKeyring.Active()
		})

		box.Append(useKeyring)
	}

	encryptFile := gtk.NewCheckButtonWithLabel("Encrypted File")
	encryptFile.Connect("toggled", func(encryptFile *gtk.CheckButton) {
		if !encryptFile.Active() {
			state.encrypt = false
			return
		}

		if a.encrypt != nil {
			// Password already provided.
			state.encrypt = true
			return
		}

		passEntry := gtk.NewEntry()
		passEntry.SetInputPurpose(gtk.InputPurposePassword)
		passEntry.SetVisibility(false)

		passLabel := gtk.NewLabel("Enter new password:")
		passLabel.SetAttributes(inputLabelAttrs)
		passLabel.SetXAlign(0)

		passBox := gtk.NewBox(gtk.OrientationVertical, 0)
		passBox.Append(passLabel)
		passBox.Append(passEntry)

		// Ask for encryption.
		passPrompt := gtk.NewDialog()
		passPrompt.SetTitle("Encrypt File")
		passPrompt.SetDefaultSize(250, 80)
		passPrompt.SetTransientFor(a.Window)
		passPrompt.SetModal(true)
		passPrompt.AddButton("Cancel", int(gtk.ResponseCancel))
		passPrompt.AddButton("Encrypt", int(gtk.ResponseAccept))
		passPrompt.SetDefaultResponse(int(gtk.ResponseAccept))

		passInner := passPrompt.ContentArea()
		passInner.Append(passBox)
		passInner.SetVExpand(true)
		passInner.SetHExpand(true)
		passInner.SetVAlign(gtk.AlignCenter)
		passInner.SetHAlign(gtk.AlignCenter)
		rememberMePasswordCSS(passInner)

		passEntry.Connect("activate", func() {
			// Enter key activates.
			passPrompt.Response(int(gtk.ResponseAccept))
		})

		passPrompt.Connect("response", func(passPrompt *gtk.Dialog, id int) {
			defer passPrompt.Close()

			password := passEntry.Text()
			if id == int(gtk.ResponseCancel) || password == "" {
				encryptFile.SetActive(false)
				state.encrypt = false
				return
			}

			if id == int(gtk.ResponseAccept) {
				a.encrypt = secret.EncryptedFileDriver(password, encryptionPath)
				state.encrypt = true
				return
			}
		})
		passPrompt.Show()
	})

	box.Append(encryptFile)

	revealer := gtk.NewRevealer()
	revealer.SetRevealChild(false)
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	revealer.SetChild(box)

	rememberMe := gtk.NewCheckButtonWithLabel("Remember Me")
	rememberMe.Connect("toggled", func(rememberMe *gtk.CheckButton) {
		isActive := rememberMe.Active()
		revealer.SetRevealChild(isActive)

		if !isActive {
			// Reset the state.
			state.encrypt = false
			state.keyring = false

			encryptFile.SetActive(false)
			if useKeyring != nil {
				useKeyring.SetActive(false)
			}
		}
	})

	state.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	state.Box.Append(rememberMe)
	state.Box.Append(revealer)
	rememberMeCSS(state.Box)

	return &state
}

func (r *rememberMeBox) saveAndFinish(a *Assistant, acc *Account) {
	go func() {
		var errors []error

		if r.keyring && a.keyring != nil {
			if err := saveAccount(a.keyring, acc); err != nil {
				errors = append(errors, err)
			}
		}

		if r.encrypt && a.encrypt != nil {
			if err := saveAccount(a.encrypt, acc); err != nil {
				errors = append(errors, err)
			}
		}

		glib.IdleAdd(func() {
			errpopup.Show(a.Window, errors, func() {
				a.Continue()
				a.finish(acc)
			})
		})
	}()
}
