package selfbar

import (
	"context"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/gotk3/gotk3/glib"
)

// Bar describes a self bar widget.
type Bar struct {
	*gtk.Box
	app *app.Application

	avatar    *adw.Avatar
	name      *gtk.Label
	hamburger *gtk.Button
}

var avatarSize = 38

var barCSS = cssutil.Applier("selfbar-bar", `
`)

// New creates a new self bar instance.
func New(app *app.Application) *Bar {
	burger := gtk.NewButtonFromIconName("open-menu-symbolic")
	burger.SetTooltipText("Menu")
	burger.AddCSSClass("selfbar-hamburger")
	burger.Connect("clicked", func() { openHamburger(app) })

	uID, _ := app.Client.Offline().Whoami()
	username, _, _ := uID.Parse()

	avatar := adw.NewAvatar(avatarSize, username, true)

	name := gtk.NewLabel("")
	name.SetHExpand(true)
	name.SetXAlign(0)
	name.SetMarkup(mauthor.Markup(
		app.Client.Offline(), "", uID,
		mauthor.WithWidgetColor(name),
	))

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(&avatar.Widget)
	box.Append(name)
	box.Append(burger)

	return &Bar{
		Box: box,
		app: app,

		avatar:    avatar,
		name:      name,
		hamburger: burger,
	}
}

// Invalidate invalidates the data displayed on the bar and refetches
// everything.
func (b *Bar) Invalidate() {
	opt := mauthor.WithWidgetColor(b.name)

	go func() {
		u, err := b.app.Client.Whoami()
		if err != nil {
			return // weird
		}

		markup := mauthor.Markup(b.app.Client, "", u, opt)
		glib.IdleAdd(func() { b.name.SetMarkup(markup) })

		mxc, _ := b.app.Client.AvatarURL(u)
		if mxc != nil {
			url, _ := b.app.Client.SquareThumbnail(*mxc, avatarSize)
			imgutil.AsyncGET(context.Background(), url, b.avatar.SetCustomImage)
		}
	}()
}

func openHamburger(app *app.Application) {
	// TODO
	// gtk.NewPopoverMenuFromModel()
}
