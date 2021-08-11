package room

import (
	"context"
	"fmt"
	"html"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

// AvatarSize is the size in pixels of the avatar.
const AvatarSize = 32

// // ShowMessagePreview determines if a room shows a preview of its latest
// // message.
// var ShowMessagePreview = prefs.NewBool(true, prefs.PropMeta{
// 	Name:        "Preview Message",
// 	Description: "Whether or not to show a preview of the latest message.",
// })

// Room is a single room row.
type Room struct {
	*gtk.ListBoxRow
	box *gtk.Box

	name    *gtk.Label
	preview *gtk.Label
	avatar  *adw.Avatar

	ID   matrix.RoomID
	Name string

	ctx     gtkutil.ContextTaker
	section Section

	showPreview bool
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

// Section is the controller interface that Room holds as its parent section.
type Section interface {
	Tag() matrix.TagName
	Reminify()
	InvalidateSort()

	Remove(*Room)
	Insert(*Room)

	OpenRoom(matrix.RoomID)
	OpenRoomInTab(matrix.RoomID)
}

// AddTo adds an empty room with the given ID to the given section..
func AddTo(ctx context.Context, section Section, roomID matrix.RoomID) *Room {
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

	r := Room{
		ListBoxRow: row,
		box:        box,
		name:       nameLabel,
		preview:    previewLabel,
		avatar:     adwAvatar,

		ctx:     gtkutil.WithWidgetVisibility(ctx, row),
		section: section,

		ID:   roomID,
		Name: string(roomID),
	}

	section.Insert(&r)

	gtkutil.BindActionMap(row, "room", map[string]func(){
		"open":        func() { section.OpenRoom(roomID) },
		"open-in-tab": func() { section.OpenRoomInTab(roomID) },
		// TODO: prompt-order
	})

	client := gotktrix.FromContext(r.ctx).Offline()

	gtkutil.BindRightClick(row, func() {
		actions := [][2]string{
			{"Open", "room.open"},
			{"Open in New Tab", "room.open-in-tab"},
		}

		gtkutil.ShowPopoverMenu(row, gtk.PosBottom, actions)
	})

	// Bind the message handler to update itself.
	gtkutil.MapSubscriber(row, func() func() {
		return client.SubscribeTimeline(roomID, func(event.RoomEvent) {
			glib.IdleAdd(func() {
				r.InvalidatePreview()
				r.section.InvalidateSort()
			})
		})
	})

	// Initialize drag-and-drop.
	drag := gtkutil.NewDragSourceWithContent(row, gdk.ActionMove, string(roomID))
	r.AddController(drag)

	return &r
}

// Section returns the current section that the room is in.
func (r *Room) Section() Section {
	return r.section
}

// IsIn returns true if the room is in the given section.
func (r *Room) IsIn(s Section) bool {
	return r.section == s
}

// Move moves the room to the given section.
func (r *Room) Move(dst Section) {
	r.section.Remove(r)
	r.section = dst
	r.section.Insert(r)
}

// Changed marks the row as changed, invalidating its sorting and filter.
func (r *Room) Changed() {
	r.ListBoxRow.Changed()
	r.section.Reminify()
}

func (r *Room) SetLabel(text string) {
	r.Name = text
	r.name.SetLabel(text)
	r.name.SetTooltipText(text)
	r.avatar.SetName(text)
	r.avatar.SetTooltipText(text)
}

// SetAvatar sets the room's avatar URL.
func (r *Room) SetAvatarURL(mxc matrix.URL) {
	client := gotktrix.FromContext(r.ctx).Offline()
	url, _ := client.SquareThumbnail(mxc, AvatarSize)
	imgutil.AsyncGET(r.ctx.Take(), url, r.avatar.SetCustomImage)
}

// SetShowMessagePreview sets whether or not the room should show the message
// preview.
func (r *Room) SetShowMessagePreview(show bool) {
	r.showPreview = show
	r.InvalidatePreview()
}

func (r *Room) erasePreview() {
	r.preview.SetLabel("")
	r.preview.Hide()
}

// InvalidatePreview invalidate the room's preview.
func (r *Room) InvalidatePreview() {
	if !r.showPreview {
		r.erasePreview()
		return
	}

	client := gotktrix.FromContext(r.ctx).Offline()

	events, err := client.RoomTimeline(r.ID)
	if err != nil || len(events) == 0 {
		r.erasePreview()
		return
	}

	preview := generatePreview(client, r.ID, events[len(events)-1])
	r.preview.SetMarkup(preview)
	r.preview.SetTooltipMarkup(preview)
	r.preview.Show()
}

func generatePreview(c *gotktrix.Client, rID matrix.RoomID, ev event.RoomEvent) string {
	name, _ := c.MemberName(rID, ev.Sender())

	switch ev := ev.(type) {
	case event.RoomMessageEvent:
		return fmt.Sprintf("%s: %s", name.Name, html.EscapeString(trimString(ev.Body, 256)))
	default:
		return fmt.Sprintf("<i>%s %s</i>", name.Name, message.EventMessageTail(ev))
	}
}

func trimString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
