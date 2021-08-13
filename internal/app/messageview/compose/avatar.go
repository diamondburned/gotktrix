package compose

import (
	"context"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

const (
	AvatarSize  = 36
	AvatarWidth = 36 + 10*2
)

// Avatar is the clickable avatar component.
type Avatar struct {
	*gtk.Button
	avatar *adw.Avatar

	ctx context.Context
}

var avatarCSS = cssutil.Applier("composer-avatar", `
	.composer-avatar {
	}
`)

// NewAvatar creates a new avatar.
func NewAvatar(ctx context.Context, name string) *Avatar {
	avatar := adw.NewAvatar(AvatarSize, name, true)

	button := gtk.NewButton()
	button.SetChild(&avatar.Widget)
	button.SetHasFrame(false)
	button.SetTooltipText(name)
	button.SetSizeRequest(AvatarWidth, -1)
	avatarCSS(button)

	return &Avatar{
		Button: button,
		avatar: avatar,
	}
}

// SetAvatarURL sets the avatar URL of this instance.
func (a *Avatar) SetAvatarURL(mxc matrix.URL) {
	client := gotktrix.FromContext(a.ctx).Offline()
	url, _ := client.SquareThumbnail(mxc, AvatarSize)
	imgutil.AsyncGET(a.ctx, url, a.avatar.SetCustomImage)
}

// SetName sets the name in plain text of the current user.
func (a *Avatar) SetName(name string) {
	a.avatar.SetText(name)
	a.Button.SetTooltipText(name)
}
