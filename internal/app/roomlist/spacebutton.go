package roomlist

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/roomlist/room"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/matrix"
)

type spaceButton interface {
	gtk.Widgetter
	SetActive(bool)
}

const spaceIconSize = 28

var spaceNameCSS = cssutil.Applier("roomlist-space-name", `
	.roomlist-space-name {
		font-size: 0.9em;
		padding-right: 4px;
	}
`)

// AllRoomsButton describes the button that says "All rooms".
type AllRoomsButton struct {
	*gtk.ToggleButton
	name *gtk.Revealer
}

var _ spaceButton = (*AllRoomsButton)(nil)

// NewAllRoomsButton creates a new AllRoomsButton.
func NewAllRoomsButton(ctx context.Context) *AllRoomsButton {
	b := AllRoomsButton{}

	label := gtk.NewLabel(locale.S(ctx, "All Rooms"))
	spaceNameCSS(label)

	b.name = gtk.NewRevealer()
	b.name.SetChild(label)
	b.name.SetRevealChild(false)
	b.name.SetTransitionType(gtk.RevealerTransitionTypeSlideRight)

	icon := gtk.NewImageFromIconName("go-home-symbolic")
	// Shrink the width a bit, since the icon has a lot of empty space around
	// it.
	icon.SetSizeRequest(spaceIconSize-2, spaceIconSize)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(icon)
	box.Append(b.name)

	b.ToggleButton = gtk.NewToggleButton()
	b.AddCSSClass("roomlist-allrooms")
	b.SetChild(box)
	b.ConnectToggled(func() { b.name.SetRevealChild(b.Active()) })

	return &b
}

// SpaceButton describes a button of a space.
type SpaceButton struct {
	*gtk.ToggleButton
	box *gtk.Box

	icon *onlineimage.Avatar
	name struct {
		*gtk.Revealer
		label *gtk.Label
	}

	state *room.State
}

var _ spaceButton = (*SpaceButton)(nil)

var spaceButtonCSS = cssutil.Applier("roomlist-space", `
	.roomlist-space {
		margin-left: 6px;
	}
	.roomlist-space .roomlist-space-name {
		padding-left: 6px;
	}
`)

// NewSpaceButton creates a new space button.
func NewSpaceButton(ctx context.Context, spaceID matrix.RoomID) *SpaceButton {
	b := SpaceButton{}

	b.name.label = gtk.NewLabel(string(spaceID))
	b.name.label.SetMaxWidthChars(65)
	b.name.label.SetEllipsize(pango.EllipsizeEnd)
	b.name.label.SetSingleLineMode(true)
	spaceNameCSS(b.name.label)

	b.name.Revealer = gtk.NewRevealer()
	b.name.SetChild(b.name.label)
	b.name.SetRevealChild(false)
	b.name.SetTransitionType(gtk.RevealerTransitionTypeSlideRight)

	b.icon = onlineimage.NewAvatar(ctx, gotktrix.AvatarProvider, spaceIconSize)

	b.box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	b.box.SetOverflow(gtk.OverflowHidden)
	b.box.Append(b.icon)
	b.box.Append(b.name)

	b.ToggleButton = gtk.NewToggleButton()
	b.ToggleButton.SetChild(b.box)
	b.ToggleButton.ConnectToggled(func() { b.name.SetRevealChild(b.Active()) })
	spaceButtonCSS(b.ToggleButton)

	b.state = room.NewState(ctx, spaceID)
	b.state.NotifyName(func(ctx context.Context, s room.State) {
		b.name.label.SetText(s.Name)
		b.box.SetTooltipText(s.Name)
	})
	b.state.NotifyAvatar(func(ctx context.Context, s room.State) {
		b.icon.SetFromURL(string(s.Avatar))
	})

	gtkutil.BindSubscribe(b, func() func() {
		return b.state.Subscribe()
	})

	return &b
}

// SpaceID returns the space ID of the button. If this returns an empty string,
// then assume it returned no space (or all rooms).
func (b *SpaceButton) SpaceID() matrix.RoomID {
	return b.state.ID
}
