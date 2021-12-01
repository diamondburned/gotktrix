package selfbar

import (
	"context"
	"fmt"
	"html"
	"strings"
	"unicode"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

// Bar describes a self bar widget.
type Bar struct {
	*gtk.Box
	ctx    context.Context
	client *gotktrix.Client

	search *gtk.SearchBar
	bar    *actionBar
}

type actionBar struct {
	*gtk.Box

	avatar *adaptive.Avatar
	name   *gtk.Label

	search *gtk.ToggleButton
	burger *gtk.Button

	actions   *gio.SimpleActionGroup
	menuItems [][2]string // id->label
}

var avatarSize = 32

var barCSS = cssutil.Applier("selfbar-bar", `
	.selfbar-bar {
		box-shadow: 0 0 8px 0px rgba(0, 0, 0, 0.35);
		background-color: @theme_bg_color;
		border: 0;
		border-radius: 0;
		padding: 0 8px;
		border:  0;
	}
	.selfbar-bar label {
		margin-left: 6px;
		font-weight: initial;
	}
	.selfbar-bar button {
		margin-left: 2px;
	}
	.selfbar-bar button:not(:hover):not(:checked) {
		background: none;
		box-shadow: none;
	}
`)

var roomSearchCSS = cssutil.Applier("selfbar-roomsearch", `
	.selfbar-roomsearch > revealer > box {
		border-bottom: 0;
		border-top: 1px solid @borders;
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
	printer := locale.Printer(ctx)

	bar := &actionBar{}
	bar.burger = gtk.NewButtonFromIconName("open-menu-symbolic")
	bar.burger.SetTooltipText(printer.Sprint("Menu"))
	bar.burger.AddCSSClass("selfbar-icon")
	bar.burger.SetVAlign(gtk.AlignCenter)

	bar.search = gtk.NewToggleButton()
	bar.search.SetIconName("system-search-symbolic")
	bar.search.SetTooltipText(printer.Sprint("Search Room"))
	bar.search.AddCSSClass("selfbar-search")
	bar.search.SetVAlign(gtk.AlignCenter)

	client := gotktrix.FromContext(ctx)

	uID, _ := client.Offline().Whoami()
	username, _, _ := uID.Parse()

	bar.avatar = adaptive.NewAvatar(avatarSize)
	bar.avatar.SetInitials(username)

	bar.name = gtk.NewLabel("")
	bar.name.SetAttributes(nameAttrs)
	bar.name.SetEllipsize(pango.EllipsizeEnd)
	bar.name.SetHExpand(true)
	bar.name.SetXAlign(0)
	bar.name.SetMarkup(nameMarkup(client.Offline(), uID, mauthor.WithWidgetColor(bar.name)))

	bar.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	bar.Box.Append(bar.avatar)
	bar.Box.Append(bar.name)
	bar.Box.Append(bar.search)
	bar.Box.Append(bar.burger)
	barCSS(bar)

	bar.actions = gio.NewSimpleActionGroup()
	bar.InsertActionGroup("selfbar", bar.actions)

	searchEntry := gtk.NewSearchEntry()
	searchEntry.SetHExpand(true)
	searchEntry.SetObjectProperty("placeholder-text", "Search Rooms")
	searchEntry.ConnectSearchChanged(func() {
		ctrl.SearchRoom(searchEntry.Text())
	})

	search := gtk.NewSearchBar()
	search.SetChild(searchEntry)
	search.ConnectEntry(&searchEntry.Editable)
	search.SetSearchMode(false)
	search.SetShowCloseButton(false)
	search.Connect("notify::search-mode-enabled", func() {
		searching := search.SearchMode()
		bar.search.SetActive(searching)

		if !searching {
			ctrl.SearchRoom("")
		}
	})
	roomSearchCSS(search)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(search)
	box.Append(bar)

	b := Bar{
		Box:    box,
		ctx:    ctx,
		client: client,
		bar:    bar,
		search: search,
	}

	bar.burger.ConnectClicked(func() {
		p := gtkutil.NewPopoverMenu(b.bar, gtk.PosTop, b.bar.menuItems)
		p.SetOffset(0, -8) // move it up a bit
		p.SetHasArrow(false)
		p.SetSizeRequest(200, -1)
		p.Popup()
	})

	bar.search.ConnectClicked(func() {
		b.search.SetSearchMode(bar.search.Active())
	})

	return &b
}

// SetSearchCaptureWidget sets the widget to capture keypresses in that will
// automatically activate room searching.
func (b *Bar) SetSearchCaptureWidget(widget gtk.Widgetter) {
	b.search.SetKeyCaptureWidget(widget)
}

// AddButton adds a button into the bar.
func (b *Bar) AddButton(label string, f func()) {
	id := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsUpper(r):
			return unicode.ToLower(r)
		case unicode.IsLower(r):
			return r
		default:
			return '-'
		}
	}, label)

	b.bar.actions.AddAction(gtkutil.ActionFunc(id, f))
	b.bar.menuItems = append(b.bar.menuItems, [2]string{label, "selfbar." + id})
}

// Invalidate invalidates the data displayed on the bar and refetches
// everything.
func (b *Bar) Invalidate() {
	opt := mauthor.WithWidgetColor(b.bar.name)

	go func() {
		u, err := b.client.Whoami()
		if err != nil {
			return // weird
		}

		markup := nameMarkup(b.client, u, opt)
		glib.IdleAdd(func() { b.bar.name.SetMarkup(markup) })

		mxc, _ := b.client.AvatarURL(u)
		if mxc != nil {
			url, _ := b.client.SquareThumbnail(*mxc, avatarSize, gtkutil.ScaleFactor())
			imgutil.AsyncGET(b.ctx, url, b.bar.avatar.SetFromPaintable)
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
