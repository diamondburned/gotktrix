package userbutton

import (
	"context"
	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
)

// Toggle is a toggle button showing the user avatar. It shows a PopoverMenu
// when clicked.
type Toggle struct {
	*gtk.ToggleButton
	MenuItems []gtkutil.PopoverMenuItem

	avatar *onlineimage.Avatar
	ctx    context.Context

	menuFn    func() []gtkutil.PopoverMenuItem
	popoverFn func(*gtk.PopoverMenu)
}

//go:embed styles/userbutton-toggle.css
var toggleStyle string
var toggleCSS = cssutil.Applier("userbutton-toggle", toggleStyle)

// NewToggle creates a new Toggle instance. It takes parameters similar to
// NewPopover.
func NewToggle(ctx context.Context) *Toggle {
	t := Toggle{ctx: ctx}

	username, _, _ := gotktrix.FromContext(ctx).UserID.Parse()

	t.avatar = onlineimage.NewAvatar(ctx, gotktrix.AvatarProvider, 32)
	t.avatar.SetInitials(username)

	t.ToggleButton = gtk.NewToggleButton()
	t.SetChild(t.avatar)
	t.ConnectClicked(func() {
		if t.menuFn == nil {
			t.SetActive(false)
			return
		}

		popover := gtkutil.NewPopoverMenuCustom(nil, gtk.PosBottom, t.menuFn())
		popover.ConnectHide(func() { t.SetActive(false) })

		if t.popoverFn != nil {
			t.popoverFn(popover)
		}

		gtkutil.PopupFinally(popover)
	})
	toggleCSS(t)

	t.InvalidateAvatar()

	return &t
}

// InvalidateAvatar updates the avatar. NewToggle will call this function on its
// own.
func (t *Toggle) InvalidateAvatar() {
	client := gotktrix.FromContext(t.ctx)

	gtkutil.Async(t.ctx, func() func() {
		mxc, _ := client.AvatarURL(client.UserID)
		if mxc == nil {
			return func() { t.avatar.SetFromURL("") }
		}
		return func() { t.avatar.SetFromURL(string(*mxc)) }
	})
}

// SetMenuFunc sets the menu function. The function is invoked everytime the
// PopoverMenu is created.
func (t *Toggle) SetMenuFunc(f func() []gtkutil.PopoverMenuItem) {
	t.menuFn = f
}

// SetPopoverFunc sets the function to be called when a Popover is spawned.
func (t *Toggle) SetPopoverFunc(f func(*gtk.PopoverMenu)) {
	t.popoverFn = f
}
