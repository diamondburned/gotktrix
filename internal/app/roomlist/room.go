package roomlist

import (
	"fmt"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// AvatarSize is the size in pixels of the avatar.
const AvatarSize = 32

// ShowMessagePreview determines if a room shows a preview of its latest
// message.
var ShowMessagePreview = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Preview Message",
	Description: "Whether or not to show a preview of the latest message.",
})

// Room is a single room row.
type Room struct {
	*gtk.ListBoxRow
	Box *gtk.Box

	Name    *gtk.Label
	Preview *gtk.Label
	Avatar  *adw.Avatar

	id      matrix.RoomID
	name    string
	section *Section
}

var avatarCSS = cssutil.Applier("roomlist-avatar", `
	.roomlist-avatar {}
`)

var roomBoxCSS = cssutil.Applier("roomlist-roombox", `
	.roomlist-roombox {
		padding: 2px 6px;
	}
	.roomlist-roomright {
		margin-left: 6px;
	}
	.roomlist-roompreview {
		font-size: 0.8em;
		color: alpha(@theme_fg_color, 0.9);
	}
`)

// AddEmptyRoom adds an empty room with the given ID.
func AddEmptyRoom(section *Section, roomID matrix.RoomID) *Room {
	nameLabel := gtk.NewLabel(string(roomID))
	nameLabel.SetSingleLineMode(true)
	nameLabel.SetXAlign(0)
	nameLabel.SetHExpand(true)
	nameLabel.SetEllipsize(pango.EllipsizeEnd)
	nameLabel.AddCSSClass("roomlist-roomname")

	previewLabel := gtk.NewLabel("")
	previewLabel.SetSingleLineMode(true)
	previewLabel.SetXAlign(0)
	previewLabel.SetHExpand(true)
	previewLabel.SetEllipsize(pango.EllipsizeEnd)
	previewLabel.Hide()
	previewLabel.AddCSSClass("roomlist-roompreview")

	rightBox := gtk.NewBox(gtk.OrientationVertical, 0)
	rightBox.SetVAlign(gtk.AlignCenter)
	rightBox.Append(nameLabel)
	rightBox.Append(previewLabel)
	rightBox.AddCSSClass("roomlist-roomright")

	adwAvatar := adw.NewAvatar(AvatarSize, string(roomID), false)
	avatarCSS(&adwAvatar.Widget)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(&adwAvatar.Widget)
	box.Append(rightBox)
	roomBoxCSS(box)

	row := gtk.NewListBoxRow()
	row.SetChild(box)
	row.SetName(string(roomID))

	section.List.Insert(row, -1)

	gtkutil.BindActionMap(row, "room", map[string]func(){
		"open":        func() { section.parent.app.OpenRoom(roomID) },
		"open-in-tab": func() { section.parent.app.OpenRoomInTab(roomID) },
	})

	gtkutil.BindPopoverMenu(row, [][2]string{
		{"Open", "room.open"},
		{"Open in New Tab", "room.open-in-tab"},
	})

	r := Room{
		ListBoxRow: row,
		Box:        box,
		Name:       nameLabel,
		Preview:    previewLabel,
		Avatar:     adwAvatar,

		id:      roomID,
		name:    string(roomID),
		section: section,
	}

	ShowMessagePreview.Connect(r, func() {
		r.InvalidatePreview()
	})

	return &r
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

func (r *Room) erasePreview() {
	r.Preview.SetLabel("")
	r.Preview.Hide()
}

// InvalidatePreview invalidate the room's preview.
func (r *Room) InvalidatePreview() {
	if !ShowMessagePreview.Value() {
		r.erasePreview()
		return
	}

	client := r.section.parent.client.Offline()

	events, err := client.RoomTimeline(r.id)
	if err != nil || len(events) == 0 {
		r.erasePreview()
		return
	}

	preview := generatePreview(client, r.id, events[len(events)-1])
	r.Preview.SetLabel(preview)
	r.Preview.Show()
}

func generatePreview(c *gotktrix.Client, rID matrix.RoomID, ev event.RoomEvent) string {
	name, _ := c.MemberName(rID, ev.Sender())

	switch ev := ev.(type) {
	case event.RoomMessageEvent:
		return fmt.Sprintf("%s: %s", name.Name, trimString(ev.Body, 256))
	default:
		return fmt.Sprintf("%s: %s", name.Name, ev.Type())
	}
}

func trimString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
