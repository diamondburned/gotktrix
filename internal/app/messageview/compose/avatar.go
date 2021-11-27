package compose

import (
	"context"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
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
	avatar *adaptive.Avatar

	menuItems func() []gtkutil.PopoverMenuItem

	rID matrix.RoomID
	ctx gtkutil.Cancellable
}

var avatarCSS = cssutil.Applier("composer-avatar", `
	.composer-avatar {
		padding-left: 12px;
		padding-right: 8px;
		border-radius: 0;
	}
	.composer-avatar,
	.composer-avatar:hover {
		background: none;
	}
	.composer-avatar:hover {
		filter: brightness(150%) contrast(50%);
	}
	.composer-avatar:active {
		margin-bottom: -3px;
	}
`)

var avatarRelevantEvents = []event.Type{
	event.TypeRoomMember, // check state key
}

// NewAvatar creates a new avatar.
func NewAvatar(ctx context.Context, roomID matrix.RoomID) *Avatar {
	client := gotktrix.FromContext(ctx).Offline()
	uID, _ := client.Whoami()

	avy := adaptive.NewAvatar(AvatarSize)
	avy.SetInitials(strings.TrimPrefix(string(uID), "@"))

	button := gtk.NewButton()
	button.SetChild(&avy.Widget)
	button.SetHasFrame(false)
	button.SetSizeRequest(AvatarWidth, -1)
	avatarCSS(button)

	avatar := Avatar{
		Button: button,
		avatar: avy,
		ctx:    gtkutil.WithCanceller(ctx),
		rID:    roomID,
	}

	invalidate := func() {
		avatar.ctx.Renew()
		avatar.invalidate()
	}

	gtkutil.MapSubscriber(button, func() func() {
		invalidate()

		return client.SubscribeRoomStateKey(
			roomID, event.TypeRoomMember, string(uID),
			func() { glib.IdleAdd(invalidate) },
		)
	})

	button.Connect("clicked", func() {
		if avatar.menuItems == nil {
			return
		}
		gtkutil.ShowPopoverMenuCustom(button, gtk.PosTop, avatar.menuItems())
	})

	return &avatar
}

func (a *Avatar) invalidate() {
	client := gotktrix.FromContext(a.ctx).Offline()
	uID, _ := client.Whoami()

	markup := mauthor.Markup(client, a.rID, uID, mauthor.WithMinimal())
	a.Button.SetTooltipMarkup(markup)

	avy, _ := client.MemberAvatar(a.rID, uID)
	if avy != nil {
		url, _ := client.SquareThumbnail(*avy, AvatarSize, gtkutil.ScaleFactor())
		imgutil.AsyncGET(a.ctx.Take(), url, a.avatar.SetFromPaintable)
	} else {
		a.avatar.SetFromPaintable(nil)
	}
}

// MenuItemsFunc sets the callback used to get menu items.
func (a *Avatar) MenuItemsFunc(items func() []gtkutil.PopoverMenuItem) {
	a.menuItems = items
}
