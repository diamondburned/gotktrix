package auth

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/pkg/errors"
)

var homeserverStepCSS = cssutil.Applier("auth-homeserver-step", ``)

func homeserverStep(a *Assistant) *assistant.Step {
	inputBox, inputs := a.makeInputs("Homeserver")
	inputs[0].SetText("matrix.org")

	errLabel := makeErrorLabel()
	errLabel.Hide()

	step := assistant.NewStep("Homeserver", "Connect")
	step.CanBack = true

	content := step.ContentArea()
	content.SetOrientation(gtk.OrientationVertical)
	content.Append(inputBox)
	content.Append(errLabel)
	homeserverStepCSS(content)

	step.Done = func(step *assistant.Step) {
		ctx := a.CancellableBusy(a.ctx)

		go func() {
			onErr := func(err error) {
				glib.IdleAdd(func() {
					errLabel.SetMarkup(textutil.ErrorMarkup(err.Error()))
					errLabel.Show()
					a.Continue()
				})
			}

			c, err := gotktrix.Discover(inputs[0].Text(), gotktrix.Opts{
				Client:     a.client.WithContext(ctx),
				ConfigPath: app.FromContext(ctx),
			})
			if err != nil {
				onErr(err)
				return
			}

			methods, err := c.LoginMethods()
			if err != nil {
				onErr(err)
				return
			}

			var pass bool
			for _, method := range methods {
				if supportedLoginMethods[method] {
					pass = true
					break
				}
			}

			if !pass {
				onErr(errors.New("no supported login methods found"))
				return
			}

			glib.IdleAdd(func() {
				c := c.WithContext(context.Background())
				a.chooseHomeserver(c, methods)
			})
		}()
	}

	return step
}
