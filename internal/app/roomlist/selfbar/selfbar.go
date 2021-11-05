package selfbar

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

// Bar describes a self bar widget.
type Bar struct {
	*gtk.Button
	ctx    context.Context
	client *gotktrix.Client

	box    *gtk.Box
	avatar *adw.Avatar
	name   *gtk.Label

	actions   *gio.SimpleActionGroup
	menuItems [][2]string // id->label
}

var avatarSize = 32

var barCSS = cssutil.Applier("selfbar-bar", `
	.selfbar-bar {
		box-shadow: 0 0 8px 0px rgba(0, 0, 0, 0.35);
		background-color: @theme_bg_color;
		border: none;
		border-radius: 0;
		padding:   0px 8px;
		padding-right: 8px;
	}
	.selfbar-bar label {
		margin-left: 6px;
		font-weight: initial;
	}
`)

type Controller interface{}

// New creates a new self bar instance.
func New(ctx context.Context, ctrl Controller) *Bar {
	printer := locale.Printer(ctx)

	burger := gtk.NewImageFromIconName("open-menu-symbolic")
	burger.SetTooltipText(printer.Sprint("Menu"))
	burger.AddCSSClass("selfbar-icon")
	burger.SetVAlign(gtk.AlignCenter)

	client := gotktrix.FromContext(ctx)

	uID, _ := client.Offline().Whoami()
	username, _, _ := uID.Parse()

	avatar := adw.NewAvatar(avatarSize, username, true)

	name := gtk.NewLabel("")
	name.SetWrap(true)
	name.SetWrapMode(pango.WrapWordChar)
	name.SetHExpand(true)
	name.SetXAlign(0)
	name.SetMarkup(nameMarkup(client.Offline(), uID, mauthor.WithWidgetColor(name)))

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(&avatar.Widget)
	box.Append(name)
	box.Append(burger)

	button := gtk.NewButton()
	button.SetHasFrame(false)
	button.SetChild(box)
	barCSS(button)

	group := gio.NewSimpleActionGroup()
	button.InsertActionGroup("selfbar", group)

	b := Bar{
		Button: button,
		ctx:    ctx,
		client: client,

		box:     box,
		avatar:  avatar,
		name:    name,
		actions: group,
	}

	button.ConnectClicked(func() {
		p := gtkutil.NewPopoverMenu(button, gtk.PosTop, b.menuItems)
		p.SetHasArrow(false)
		p.SetSizeRequest(200, -1)
		p.Popup()
	})

	return &b
}

var idReplacer = strings.NewReplacer(
	" ", "-", "\n", "-",
)

// AddButton adds a button into the bar.
func (b *Bar) AddButton(label string, f func()) {
	id := idReplacer.Replace(strings.ToLower(label))

	b.actions.AddAction(gtkutil.ActionFunc(id, f))
	b.menuItems = append(b.menuItems, [2]string{label, "selfbar." + id})
}

// Invalidate invalidates the data displayed on the bar and refetches
// everything.
func (b *Bar) Invalidate() {
	opt := mauthor.WithWidgetColor(b.name)

	go func() {
		u, err := b.client.Whoami()
		if err != nil {
			return // weird
		}

		markup := nameMarkup(b.client, u, opt)
		glib.IdleAdd(func() { b.name.SetMarkup(markup) })

		mxc, _ := b.client.AvatarURL(u)
		if mxc != nil {
			url, _ := b.client.SquareThumbnail(*mxc, avatarSize, gtkutil.ScaleFactor())
			imgutil.AsyncGET(b.ctx, url, b.avatar.SetCustomImage)
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
