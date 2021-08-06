package selfbar

import (
	"context"
	"fmt"
	"html"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
)

// Bar describes a self bar widget.
type Bar struct {
	*gtk.Box
	app *app.Application

	avatar    *adw.Avatar
	name      *gtk.Label
	hamburger *gtk.Button
}

var avatarSize = 32

var barCSS = cssutil.Applier("selfbar-bar", `
	.selfbar-bar {
		padding:   2px 8px;
		padding-right: 6px;
		box-shadow: 0 0 8px 0px rgba(0, 0, 0, 0.35);
		background-color: @theme_bg_color;
	}
	.selfbar-bar label {
		margin-left: 6px;
	}
`)

// New creates a new self bar instance.
func New(app *app.Application) *Bar {
	burger := gtk.NewButtonFromIconName("open-menu-symbolic")
	burger.SetTooltipText("Menu")
	burger.AddCSSClass("selfbar-hamburger")
	burger.SetHasFrame(false)
	burger.SetVAlign(gtk.AlignCenter)
	burger.Connect("clicked", func() { openHamburger(app) })

	uID, _ := app.Client().Offline().Whoami()
	username, _, _ := uID.Parse()

	avatar := adw.NewAvatar(avatarSize, username, true)

	name := gtk.NewLabel("")
	name.SetWrap(true)
	name.SetWrapMode(pango.WrapWordChar)
	name.SetHExpand(true)
	name.SetXAlign(0)
	name.SetMarkup(nameMarkup(app.Client().Offline(), uID, mauthor.WithWidgetColor(name)))

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(&avatar.Widget)
	box.Append(name)
	box.Append(burger)
	barCSS(box)

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
	client := b.app.Client()

	go func() {
		u, err := client.Whoami()
		if err != nil {
			return // weird
		}

		markup := nameMarkup(client, u, opt)
		glib.IdleAdd(func() { b.name.SetMarkup(markup) })

		mxc, _ := client.AvatarURL(u)
		if mxc != nil {
			url, _ := client.SquareThumbnail(*mxc, avatarSize)
			imgutil.AsyncGET(context.Background(), url, b.avatar.SetCustomImage)
		}
	}()
}

func nameMarkup(c *gotktrix.Client, uID matrix.UserID, mods ...mauthor.MarkupMod) string {
	mods = append(mods, mauthor.WithMinimal())
	markup := mauthor.Markup(c, "", uID, mods...)

	_, hostname, _ := uID.Parse()
	if hostname != "" {
		markup += "\n" + fmt.Sprintf(
			`<span size="small" fgalpha="80%%">%s</span>`,
			html.EscapeString(hostname),
		)
	}

	return markup
}

func openHamburger(app *app.Application) {
	// TODO
	// gtk.NewPopoverMenuFromModel()
}
