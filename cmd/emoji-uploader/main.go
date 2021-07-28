package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/davecgh/go-spew/spew"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/auth"
	"github.com/diamondburned/gotktrix/internal/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/emojis"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

func main() {
	app := gtk.NewApplication(config.AppIDDot("emoji-uploader"), 0)
	app.Connect("activate", activate)

	// Quit the application on a SIGINT.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go func() {
		<-ctx.Done()
		app.Quit()
	}()

	if code := app.Run(os.Args); code > 0 {
		log.Println("exit status", code)
		os.Exit(code)
	}
}

func activate(app *gtk.Application) {
	cssutil.ApplyGlobalCSS()

	spinner := gtk.NewSpinner()
	spinner.Start()
	spinner.SetSizeRequest(24, 24)

	window := gtk.NewApplicationWindow(app)
	window.SetDefaultSize(450, 300)
	window.SetTitle("Emoji Uploader")
	window.SetChild(spinner)
	window.Show()

	fiddle := func(client *gotktrix.Client, acc *auth.Account) {
		syncbox.Open(window, acc, client)

		e, err := client.WaitForEvent(context.Background(), emojis.UserEmotesEventType)
		if err != nil {
			errpopup.ShowFatal(&window.Window, []error{err})
			return
		}

		spew.Dump(e)
	}

	authAssistant := auth.New(&window.Window)
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		go fiddle(client, acc)
	})
	authAssistant.Show()
}
