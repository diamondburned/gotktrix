package main

import (
	"log"
	"os"

	"github.com/chanbakjsd/gotrix"
	"github.com/davecgh/go-spew/spew"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/auth"
	"github.com/diamondburned/gotktrix/internal/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/matrix/emojis"
)

func main() {
	app := gtk.NewApplication(config.AppIDDot("emoji-uploader"), 0)
	app.Connect("activate", activate)

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

	fiddle := func(client *gotrix.Client) {
		if err := client.Open(); err != nil {
			errpopup.Show(&window.Window, []error{err}, window.Close)
			return
		}

		v, err := client.RoomState("", emojis.UserEmotesEventType, "")
		if err != nil {
			errpopup.Show(&window.Window, []error{err}, window.Close)
			return
		}

		spew.Dump(v)

		select {}
	}

	authAssistant := auth.New(&window.Window)
	authAssistant.OnConnect(func(client *gotrix.Client, acc *auth.Account) {
		syncbox.Show(&window.Window, acc)
		go fiddle(client)
	})
	authAssistant.Show()
}
