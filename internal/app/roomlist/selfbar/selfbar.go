package selfbar

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/components/title"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

// Bar describes a self bar widget.
type Bar struct {
	*gtk.ToggleButton
	ctx    context.Context
	client *gotktrix.Client

	box *gtk.Box

	avatar *adaptive.Avatar
	name   *title.Subtitle

	actions   *gio.SimpleActionGroup
	menuItems [][2]string // id->label
}

var avatarSize = 32

var barCSS = cssutil.Applier("selfbar-bar", `
	.selfbar-bar {
		color: @theme_fg_color;
		box-shadow: 0 0 8px 0px rgba(0, 0, 0, 0.35);
		border-radius: 0;
		padding: 0 8px;
	}
	.selfbar-bar .subtitle {
		margin-left: 6px;
		font-weight: initial;
	}
`)

type Controller interface {
	SearchRoom(name string)
}

var nameAttrs = markuputil.Attrs(
	pango.NewAttrAllowBreaks(false),
	pango.NewAttrInsertHyphens(false),
)

// New creates a new self bar instance.
func New(ctx context.Context, ctrl Controller) *Bar {
	client := gotktrix.FromContext(ctx)

	bar := &Bar{
		ctx:    ctx,
		client: client,
	}

	uID, _ := client.Offline().Whoami()
	username, _, _ := uID.Parse()

	bar.name = title.NewSubtitle()
	bar.name.SetTitle(username)
	bar.name.AddCSSClass("selfbar-name")
	bar.name.SetXAlign(0)
	bar.name.SetHExpand(true)

	bar.avatar = adaptive.NewAvatar(avatarSize)
	bar.avatar.AddCSSClass("selfbar-avatar")
	bar.avatar.ConnectLabel(bar.name.Title)

	bar.box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	bar.box.Append(bar.avatar)
	bar.box.Append(bar.name)

	bar.ToggleButton = gtk.NewToggleButton()
	bar.SetHasFrame(false)
	bar.SetChild(bar.box)
	barCSS(bar)

	bar.actions = gio.NewSimpleActionGroup()
	bar.InsertActionGroup("selfbar", bar.actions)

	bar.ToggleButton.ConnectClicked(func() {
		p := gtkutil.NewPopoverMenu(bar, gtk.PosTop, bar.menuItems)
		// p.SetOffset(0, -4) // move it up a bit
		p.SetHasArrow(false)
		p.SetSizeRequest(230, -1)
		p.ConnectClosed(func() { bar.ToggleButton.SetActive(false) })
		gtkutil.PopupFinally(p)
	})

	return bar
}

// AddButton adds a button into the bar.
func (b *Bar) AddButton(label string, f func()) {
	id := gtkutil.ActionID(label)
	b.actions.AddAction(gtkutil.ActionFunc(id, f))
	b.menuItems = append(b.menuItems, [2]string{label, "selfbar." + id})
}

// Invalidate invalidates the data displayed on the bar and refetches
// everything.
func (b *Bar) Invalidate() {
	opts := []mauthor.MarkupMod{
		mauthor.WithWidgetColor(b.name),
		mauthor.WithMinimal(),
	}

	go func() {
		u, err := b.client.Whoami()
		if err != nil {
			return // weird
		}

		markup := mauthor.Markup(b.client, "", u, opts...)
		_, hostname, _ := u.Parse()

		glib.IdleAdd(func() {
			b.name.Title.SetMarkup(markup)
			b.name.SetSubtitle(hostname)
		})

		mxc, _ := b.client.AvatarURL(u)
		if mxc != nil {
			url, _ := b.client.SquareThumbnail(*mxc, avatarSize, gtkutil.ScaleFactor())
			imgutil.AsyncGET(b.ctx, url, b.avatar.SetFromPaintable)
		}
	}()
}
