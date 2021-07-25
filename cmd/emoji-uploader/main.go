package main

import (
	"log"
	"os"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/auth"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
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
	window.SetTitle("Emoji Uploader")
	window.SetChild(spinner)
	window.Show()

	authAssistant := auth.New(&window.Window)
	authAssistant.Show()
}
