package auth

import (
	"context"
	"log"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/errpopup"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/secret"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

func loginStep(a *Assistant, method matrix.LoginMethod) *assistant.Step {
	switch method {
	case matrix.LoginSSO:
		return loginStepSSO(a)
	case matrix.LoginPassword, matrix.LoginToken:
		return loginStepForm(a, method)
	default:
		log.Panicln("unknown login method", method)
		return nil
	}
}

type loginStepData struct {
	InputBox gtk.Widgetter
	Login    func(client *gotktrix.ClientAuth) (*gotktrix.Client, error)
}

func loginStepForm(a *Assistant, method matrix.LoginMethod) *assistant.Step {
	var data loginStepData

	switch method {
	case matrix.LoginPassword:
		inputBox, inputs := a.makeInputs("Username (or Email)", "Password")
		inputs[0].SetInputPurpose(gtk.InputPurposeEmail)
		inputs[1].SetInputPurpose(gtk.InputPurposePassword)
		inputs[1].SetVisibility(false)

		data.InputBox = inputBox
		data.Login = func(client *gotktrix.ClientAuth) (*gotktrix.Client, error) {
			return client.LoginPassword(inputs[0].Text(), inputs[1].Text())
		}
	case matrix.LoginToken:
		inputBox, inputs := a.makeInputs("Token")
		inputs[0].SetInputPurpose(gtk.InputPurposePassword)
		inputs[0].SetVisibility(false)

		data.InputBox = inputBox
		data.Login = func(client *gotktrix.ClientAuth) (*gotktrix.Client, error) {
			return client.LoginToken(inputs[0].Text())
		}
	}

	errLabel := makeErrorLabel()
	errLabel.Hide()

	rememberMe := newRememberMeBox(a)

	step := assistant.NewStep("Password", "Log in")
	step.CanBack = true

	content := step.ContentArea()
	content.SetOrientation(gtk.OrientationVertical)
	content.Append(data.InputBox)
	content.Append(errLabel)
	content.Append(rememberMe)

	onError := func(err error) {
		errLabel.SetMarkup(textutil.ErrorMarkup(err.Error()))
		errLabel.Show()
		a.Continue()
	}

	step.Done = func(step *assistant.Step) {
		ctx := a.CancellableBusy(a.ctx)

		go func() {
			client := a.currentClient.WithContext(ctx)

			c, err := data.Login(client)
			if err != nil {
				glib.IdleAdd(func() { onError(err) })
				return
			}

			acc, err := copyAccount(c)
			if err != nil {
				glib.IdleAdd(func() { onError(err) })
				return
			}

			glib.IdleAdd(func() {
				// Assistant is still busy at this point.
				rememberMe.saveAndFinish(c, a, acc)
			})
		}()
	}

	return step
}

var ssoLoadingCSS = cssutil.Applier("auth-sso-loading", `
	.auth-sso-loading > label {
		margin-bottom: 16px;
	}
	.auth-sso-loading > button {
		margin-bottom: 18px;
	}
	.auth-sso-loading > .auth-remember-me {
		margin-top: 6px;
	}
`)

func loginStepSSO(a *Assistant) *assistant.Step {
	urlButton := gtk.NewLinkButtonWithLabel("", locale.S(a.ctx, "Opening your browser..."))
	urlButton.SetSensitive(false)
	urlButton.SetHAlign(gtk.AlignCenter)

	desc := gtk.NewLabel(locale.S(a.ctx, "Continue on your web browser."))
	desc.SetWrap(true)
	desc.SetWrapMode(pango.WrapWordChar)

	// TODO: maybe move this into its own step.
	rememberMe := newRememberMeBox(a)

	loading := gtk.NewBox(gtk.OrientationVertical, 0)
	loading.AddCSSClass("assistant-stepbody") // hax
	loading.SetSizeRequest(200, -1)
	loading.SetHAlign(gtk.AlignCenter)
	loading.SetVAlign(gtk.AlignCenter)
	loading.Append(desc)
	loading.Append(urlButton)
	loading.Append(gtk.NewSeparator(gtk.OrientationVertical))
	loading.Append(rememberMe)
	ssoLoadingCSS(loading)

	step := assistant.NewStep(locale.S(a.ctx, "SSO Login"), "")
	step.Loading = loading
	step.CanBack = true
	step.Done = assistant.MustNotDone

	onError := func(err error) {
		// Just go back directly if this is a context cancelled error, since
		// the user hit the back button.
		if errors.Is(err, context.Canceled) {
			a.GoBack()
			return
		}

		errLabel := adaptive.NewErrorLabel(err)

		content := step.ContentArea()
		content.SetOrientation(gtk.OrientationVertical)
		content.SetSpacing(4)
		content.Append(gtk.NewLabel("Unrecoverable error encountered:"))
		content.Append(errLabel)

		// Errors are unrecoverable.
		a.Continue()
	}

	done := func(c *gotktrix.Client, err error) {
		if err != nil {
			glib.IdleAdd(func() { onError(err) })
			return
		}

		acc, err := copyAccount(c)
		if err != nil {
			glib.IdleAdd(func() { onError(err) })
			return
		}

		glib.IdleAdd(func() {
			// Assistant is still busy at this point.
			rememberMe.saveAndFinish(c, a, acc)
		})
	}

	step.SwitchedTo = func(*assistant.Step) {
		ctx := a.CancellableBusy(a.ctx)

		gtkutil.Async(context.Background(), func() func() {
			client := a.currentClient.WithContext(ctx)

			address, err := client.LoginSSO(done)
			if err != nil {
				return func() { onError(errors.Wrap(err, "cannot start SSO server")) }
			}

			app.OpenURI(ctx, address)

			// Give the button 3 seconds before allowing the user to open
			// the browser again.
			glib.TimeoutAdd(3, func() {
				urlButton.SetURI(address)
				urlButton.SetLabel(locale.S(a.ctx, "Reopen the browser"))
				urlButton.SetHasFrame(true)
				urlButton.SetSensitive(true)
			})

			return nil
		})
	}

	return step
}

type rememberMeBox struct {
	*gtk.Box
	keyring bool
	encrypt bool
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
		useKeyring.ConnectToggled(func() {
			state.keyring = useKeyring.Active()
		})

		box.Append(useKeyring)
	}

	encryptFile := gtk.NewCheckButtonWithLabel("Encrypted File")
	encryptFile.ConnectToggled(func() {
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

		passEntry.ConnectActivate(func() {
			// Enter key activates.
			passPrompt.Response(int(gtk.ResponseAccept))
		})

		passPrompt.ConnectResponse(func(id int) {
			defer passPrompt.Close()

			password := passEntry.Text()
			if id == int(gtk.ResponseCancel) || password == "" {
				encryptFile.SetActive(false)
				state.encrypt = false
				return
			}

			if id == int(gtk.ResponseAccept) {
				a.encrypt = secret.EncryptedFileDriver(password, a.encryptPath)
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
	rememberMe.ConnectToggled(func() {
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

func (r *rememberMeBox) saveAndFinish(c *gotktrix.Client, a *Assistant, acc *Account) {
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
				a.finish(c, acc)
			})
		})
	}()
}
