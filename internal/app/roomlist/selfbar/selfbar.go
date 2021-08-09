package selfbar

import (
	"context"
	"fmt"
	"html"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

// Bar describes a self bar widget.
type Bar struct {
	*gtk.Button
	app Application

	box    *gtk.Box
	avatar *adw.Avatar
	name   *gtk.Label
}

var avatarSize = 32

var barCSS = cssutil.Applier("selfbar-bar", `
	.selfbar-bar {
		box-shadow: 0 0 8px 0px rgba(0, 0, 0, 0.35);
		background-color: @theme_bg_color;
		border: none;
		border-radius: 0;
		padding:   4px 8px;
		padding-right: 8px;
	}
	.selfbar-bar label {
		margin-left: 6px;
		font-weight: initial;
	}
`)

type Application interface {
	app.Applicationer
	// BeginReorderMode signals the application to put the room list under
	// reorder mode, or override mode, which would allow the user to arbitrarily
	// move rooms according to roomsort anchors.
	BeginReorderMode()
}

// New creates a new self bar instance.
func New(app Application) *Bar {
	burger := gtk.NewImageFromIconName("open-menu-symbolic")
	burger.SetTooltipText("Menu")
	burger.AddCSSClass("selfbar-icon")
	burger.SetVAlign(gtk.AlignCenter)

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

	button := gtk.NewButton()
	button.SetHasFrame(false)
	button.SetChild(box)
	barCSS(button)

	gtkutil.BindActionMap(button, "selfbar", map[string]func(){
		"begin-reorder-mode": app.BeginReorderMode,
	})
	button.Connect("clicked", func() {
		gtkutil.ShowPopoverMenu(button, gtk.PosTop, [][2]string{
			{"_Reorder Rooms", "selfbar.begin-reorder-mode"},
		})
	})

	return &Bar{
		Button: button,
		app:    app,

		box:    box,
		avatar: avatar,
		name:   name,
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
