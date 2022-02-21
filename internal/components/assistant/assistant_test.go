package assistant_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"log"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
)

type Login struct {
	*gtk.Grid
	Username *gtk.Entry
	Password *gtk.Entry
}

func NewLogin() *Login {
	username := gtk.NewEntry()
	username.SetInputPurpose(gtk.InputPurposeName)

	password := gtk.NewEntry()
	password.SetInputPurpose(gtk.InputPurposePassword)

	labels := [2]*gtk.Label{
		gtk.NewLabel("Username:"),
		gtk.NewLabel("Password:"),
	}

	labels[0].SetXAlign(1)
	labels[1].SetXAlign(1)

	grid := gtk.NewGrid()
	grid.SetVAlign(gtk.AlignCenter)
	grid.SetHAlign(gtk.AlignCenter)
	grid.SetVExpand(true)
	grid.SetHExpand(true)
	grid.SetRowSpacing(5)
	grid.SetColumnSpacing(7)
	grid.Attach(labels[0], 0, 0, 1, 1)
	grid.Attach(labels[1], 0, 1, 1, 1)
	grid.Attach(username, 1, 0, 1, 1)
	grid.Attach(password, 1, 1, 1, 1)

	return &Login{grid, username, password}
}

// Passhash hashes the password entry's content using FNV 128-bit.
func (l *Login) Passhash() string {
	hash := fnv.New128a().Sum([]byte(l.Password.Text()))
	return string(base64.StdEncoding.EncodeToString(hash))
}

func Example() {
	app := gtk.NewApplication("com.github.diamondburned.example-app", 0)
	app.ConnectActivate(func(app *gtk.Application) {
		login := NewLogin()

		window := gtk.NewApplicationWindow(app)
		window.SetDefaultSize(400, 300)
		window.SetTitle("Example Assistant.")
		window.SetChild(gtk.NewLabel("Example assistant."))
		window.Show()

		greetings := gtk.NewLabel("Please press Continue.")
		greetings.SetHExpand(true)

		onDone := func(step *assistant.Step) {
			msg := fmt.Sprintf(
				"Your username is %s.\nYour password hash is %s.",
				login.Username.Text(),
				login.Passhash(),
			)

			msgLabel := gtk.NewLabel(msg)
			msgLabel.SetWrap(true)
			msgLabel.SetWrapMode(pango.WrapWordChar)

			next := assistant.NewStep("Finish", "Done")
			next.ContentArea().Append(msgLabel)

			assistant := step.Assistant()
			assistant.AddStep(next)
			assistant.SetStep(next)
			assistant.Connect("close", window.Close)
		}

		steps := assistant.BuildSteps(
			assistant.NewStepData("Welcome", "Continue", greetings),
			assistant.StepData{
				Title:    "Authenticate",
				OKLabel:  "Login",
				Contents: []gtk.Widgetter{login},
				Done: func(step *assistant.Step) {
					assistant := step.Assistant()
					ctx := assistant.CancellableBusy(context.Background())

					go func() {
						select {
						case <-time.Tick(5 * time.Second):
							glib.IdleAdd(func() { onDone(step) })
						case <-ctx.Done():
							glib.IdleAdd(func() {
								assistant.Continue()
								assistant.Close()
							})
						}
					}()
				},
			},
		)

		a := assistant.New(&window.Window, steps)
		a.SetTitle("Hello")
		a.Connect("close-request", window.Close)
		a.Show()
	})

	if code := app.Run(nil); code > 0 {
		log.Panicf("exit status %d", code)
	}

	// Output:
}
