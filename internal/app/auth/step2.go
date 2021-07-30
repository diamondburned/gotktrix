package auth

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/gotk3/gotk3/glib"
)

var homeserverStepCSS = cssutil.Applier("auth-homeserver-step", ``)

func homeserverStep(a *Assistant) *assistant.Step {
	inputBox, inputs := a.makeInputs("Homeserver")
	inputs[0].SetText("matrix.org")

	errLabel := makeErrorLabel()
	errLabel.Hide()

	step := assistant.NewStep("Homeserver", "Connect")
	// step.CanBack = true

	content := step.ContentArea()
	content.SetOrientation(gtk.OrientationVertical)
	content.Append(inputBox)
	content.Append(errLabel)
	homeserverStepCSS(content)

	step.Done = func(step *assistant.Step) {
		ctx := a.CancellableBusy(context.Background())

		go func() {
			client := a.client.WithContext(ctx)
			c, err := gotktrix.Discover(client, inputs[0].Text())

			glib.IdleAdd(func() {
				if err == nil {
					a.chooseHomeserver(c.WithContext(context.Background()))
					return
				}

				errLabel.SetMarkup(markuputil.Error(err.Error()))
				errLabel.Show()
				a.Continue()
			})
		}()
	}

	return step
}
