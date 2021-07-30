package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/auth"
	"github.com/diamondburned/gotktrix/internal/auth/syncbox"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/roomlist"
	"github.com/gotk3/gotk3/glib"
)

func main() {
	app := gtk.NewApplication(config.AppIDDot("emoji-uploader"), 0)
	app.Connect("activate", activate)

	// Quit the application on a SIGINT.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go func() {
		<-ctx.Done()
		// Quit with high priority.
		glib.IdleAddPriority(glib.PRIORITY_HIGH, func() { app.Quit() })
	}()

	if code := app.Run(os.Args); code > 0 {
		log.Println("exit status", code)
		os.Exit(code)
	}
}

func activate(app *gtk.Application) {
	cssutil.ApplyGlobalCSS()

	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(true)

	spinner := gtk.NewSpinner()
	spinner.Start()
	spinner.SetSizeRequest(18, 18)
	spinner.SetHAlign(gtk.AlignCenter)
	spinner.SetVAlign(gtk.AlignCenter)

	window := gtk.NewApplicationWindow(app)
	window.SetDefaultSize(600, 400)
	window.SetTitle("Emoji Uploader")
	window.SetChild(spinner)
	window.SetTitlebar(header)
	window.Show()

	authAssistant := auth.New(&window.Window)
	authAssistant.Show()
	authAssistant.OnConnect(func(client *gotktrix.Client, acc *auth.Account) {
		go func() {
			popup := syncbox.Open(window, client, acc)
			popup.QueueSetLabel("Getting rooms...")

			rooms, err := client.Rooms()
			if err != nil {
				errpopup.Fatal(&window.Window, err)
				return
			}

			glib.IdleAdd(func() {
				app := &Application{
					Application: app,
					Window:      window,
					Header:      header,
					Client:      client,
				}
				ready(app, rooms)
				popup.Close()
			})
		}()
	})
}

type Application struct {
	*gtk.Application
	Window *gtk.ApplicationWindow
	Header *gtk.HeaderBar
	Client *gotktrix.Client
}

func ready(app *Application, rooms []matrix.RoomID) {
	list := roomlist.New(app.Client)
	list.AddRooms(rooms)

	listScroll := gtk.NewScrolledWindow()
	listScroll.SetHExpand(false)
	listScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	listScroll.SetChild(list)

	flap := adw.NewFlap()
	flap.SetFlap(listScroll)
	flap.SetContent(gtk.NewLabel("Hello :)"))
	flap.SetSwipeToOpen(true)
	flap.SetSwipeToClose(true)
	flap.SetFoldPolicy(adw.FlapFoldPolicyAuto)
	flap.SetTransitionType(adw.FlapTransitionTypeOver)
	flap.SetSeparator(gtk.NewSeparator(gtk.OrientationVertical))

	unflap := gtk.NewButtonFromIconName("document-properties-symbolic")
	unflap.InitiallyUnowned.Connect("clicked", func() {
		flap.SetRevealFlap(!flap.RevealFlap())
	})

	app.Window.SetChild(flap)
	app.Header.PackStart(unflap)
}
