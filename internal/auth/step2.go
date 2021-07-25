package auth

import (
	"context"

	"github.com/chanbakjsd/gotrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/gotk3/gotk3/glib"
)

var homeserverStepCSS = cssutil.Applier("auth-homeserver-step", `
	.auth-homeserver-step .error {
		padding-top: 4px;
	}
`)

func homeserverStep(a *Assistant) *assistant.Step {
	inputBox, inputs := makeInputs("Homeserver")
	inputs[0].SetPlaceholderText("matrix.org")

	errLabel := gtk.NewLabel("")
	errLabel.SetWrap(true)
	errLabel.SetWrapMode(pango.WrapWordChar)
	errLabel.SetCSSClasses([]string{"error"})
	errLabel.Hide()

	step := assistant.NewStep("Homeserver", "Connect")
	content := step.ContentArea()
	content.SetOrientation(gtk.OrientationVertical)
	content.Append(errLabel)
	content.Append(inputBox)
	homeserverStepCSS(content)

	step.Done = func(step *assistant.Step) {
		ass := step.Assistant()
		ctx := ass.CancellableBusy(context.Background())

		go func() {
			client := a.client.WithContext(ctx)
			c, err := gotrix.DiscoverWithClient(client, inputs[0].Text())

			glib.IdleAdd(func() {
				ass.Continue()

				if err == nil {
					a.chooseHomeserver(c)
					return
				}

				errLabel.SetMarkup(markuputil.Error(err.Error()))
				errLabel.Show()
			})
		}()
	}

	return step
}
