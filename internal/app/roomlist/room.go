package roomlist

import (
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// AvatarSize is the size in pixels of the avatar.
const AvatarSize = 32

// Room is a single room row.
type Room struct {
	*gtk.ListBoxRow
	Box *gtk.Box

	Name   *gtk.Label
	Avatar *adw.Avatar

	name    string
	section *Section
}

var avatarCSS = cssutil.Applier("roomlist-avatar", `
	.roomlist-avatar {
		padding: 2px 4px;
	}
`)

// AddEmptyRoom adds an empty room with the given ID.
func AddEmptyRoom(section *Section, roomID matrix.RoomID) Room {
	nameLabel := gtk.NewLabel(string(roomID))
	nameLabel.SetXAlign(0)
	nameLabel.SetEllipsize(pango.EllipsizeEnd)
	nameLabel.SetHExpand(true)

	adwAvatar := adw.NewAvatar(AvatarSize, string(roomID), false)
	avatarCSS(&adwAvatar.Widget)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(&adwAvatar.Widget)
	box.Append(nameLabel)

	row := gtk.NewListBoxRow()
	row.SetChild(box)
	row.SetName(string(roomID))

	section.List.Insert(row, -1)

	return Room{
		ListBoxRow: row,
		Box:        box,
		Name:       nameLabel,
		Avatar:     adwAvatar,
		section:    section,
	}
}

func (r *Room) move(dst *Section) {
	r.section.List.Remove(r.ListBoxRow)
	r.section = dst
	r.section.List.Insert(r.ListBoxRow, -1)
}

func (r Room) SetLabel(text string) {
	r.name = text
	r.Name.SetLabel(text)
	r.Avatar.SetName(text)
}
