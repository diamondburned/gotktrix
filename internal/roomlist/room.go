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

type room struct {
	*gtk.ListBoxRow
	name    *gtk.Label
	avatar  *adw.Avatar
	section *section
}

var avatarCSS = cssutil.Applier("roomlist-avatar", `
	.roomlist-avatar {
		padding: 2px 4px;
	}
`)

func addEmptyRoom(section *section, roomID matrix.RoomID) room {
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

	section.list.Insert(row, -1)

	return room{
		ListBoxRow: row,
		name:       nameLabel,
		avatar:     adwAvatar,
		section:    section,
	}
}

func (r *room) move(dst *section) {
	r.section.list.Remove(r)
	r.section = dst
	r.section.list.Insert(r, -1)
}

func (r *room) SetLabel(text string) {
	r.name.SetLabel(text)
	r.avatar.SetName(text)
}
