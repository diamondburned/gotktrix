package room

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/components/dialogs"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/pkg/errors"
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

	// MoveRoomToTag moves the room with the given ID to the given tag name. A
	// new section must be created if needed.
	MoveRoomToTag(src matrix.RoomID, tag matrix.TagName) bool
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
		"open":            func() { section.OpenRoom(roomID) },
		"open-in-tab":     func() { section.OpenRoomInTab(roomID) },
		"prompt-reorder":  func() { r.promptReorder() },
		"move-to-section": nil,
	})

	gtkutil.BindPopoverMenuLazy(row, gtk.PosBottom, func() []gtkutil.PopoverMenuItem {
		return []gtkutil.PopoverMenuItem{
			gtkutil.MenuItem("Open", "room.open"),
			gtkutil.MenuItem("Open in New Tab", "room.open-in-tab"),
			gtkutil.MenuSeparator("Section"),
			gtkutil.MenuItem("Reorder Room...", "room.prompt-reorder"),
			gtkutil.Submenu("Move to Section...", []gtkutil.PopoverMenuItem{
				gtkutil.MenuWidget("room.move-to-section", r.moveToSectionBox()),
			}),
		}
	})

	client := gotktrix.FromContext(r.ctx).Offline()

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

// SetOrder sets the room's order within the section it is in. If the order is
// not within [0.0, 1.0], then it is cleared.
func (r *Room) SetOrder(order float64) {
	r.SetSensitive(false)

	go func() {
		defer glib.IdleAdd(func() {
			r.SetSensitive(true)
			r.section.InvalidateSort()
		})

		client := gotktrix.FromContext(r.ctx)

		tag := matrix.Tag{}
		if order >= 0 && order <= 1 {
			tag.Order = &order
		}

		if err := client.TagAdd(r.ID, r.section.Tag(), tag); err != nil {
			app.Error(r.ctx, errors.Wrap(err, "failed to update tag"))
			return
		}

		if err := client.UpdateRoomTags(r.ID); err != nil {
			app.Error(r.ctx, errors.Wrap(err, "failed to update tag state"))
			return
		}
	}()
}

// Order returns the current room's order number, or -1 if the room doesn't have
// one.
func (r *Room) Order() float64 {
	e, err := gotktrix.FromContext(r.ctx).Offline().RoomEvent(r.ID, event.TypeTag)
	if err == nil {
		t, ok := e.(event.TagEvent).Tags[r.section.Tag()]
		if ok && t.Order != nil {
			return *t.Order
		}
	}
	return -1
}

const reorderHelp = `A room's order within a section is defined by a number
going from 0 to 1, or more precisely in interval notation, <tt>[0.0, 1.0]</tt>.
<b>Rooms with the lowest order (0.0) will be sorted before rooms with a higher
order.</b> Rooms that have the same order number will use the section's sorting
(A-Z or Activity).`

var reorderHelpAttrs = markuputil.Attrs(
	pango.NewAttrScale(0.95),
)

var reorderDialog = cssutil.Applier("room-reorderdialog", `
	.room-reorderdialog {
		padding: 15px;
	}
	.room-reorderdialog box.linked {
		margin: 10px;
	}
	.room-reorderdialog spinbutton {
		padding: 2px;
	}
`)

func (r *Room) promptReorder() {
	help := gtk.NewLabel(clean(reorderHelp))
	help.SetUseMarkup(true)
	help.SetXAlign(0)
	help.SetWrap(true)
	help.SetWrapMode(pango.WrapWordChar)
	help.SetAttributes(reorderHelpAttrs)

	spin := gtk.NewSpinButtonWithRange(0, 1, 0.05)
	spin.SetWidthChars(5) // 0.000
	spin.SetDigits(3)

	reset := gtk.NewToggleButton()
	reset.SetIconName("edit-clear-all-symbolic")
	reset.SetTooltipText("Reset")

	var resetting bool

	reset.Connect("toggled", func() {
		resetting = reset.Active()
		// Disable the spinner if we're resetting.
		spin.SetSensitive(!resetting)

		if resetting {
			reset.AddCSSClass("destructive-action")
		} else {
			reset.RemoveCSSClass("destructive-action")
		}
	})

	if order := r.Order(); order != -1 {
		spin.SetValue(order)
	} else {
		reset.SetActive(true)
	}

	inputBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	inputBox.AddCSSClass("linked")
	inputBox.SetHAlign(gtk.AlignCenter)
	inputBox.Append(spin)
	inputBox.Append(reset)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(help)
	box.Append(inputBox)
	reorderDialog(box)

	dialog := dialogs.New(app.FromContext(r.ctx).Window(), "Discard", "Save")
	dialog.SetDefaultSize(500, 225)
	dialog.SetChild(box)
	dialog.SetTitle("Reorder " + r.Name)

	dialog.Cancel.Connect("clicked", func() {
		dialog.Close()
	})

	dialog.OK.Connect("clicked", func() {
		dialog.Close()
		if resetting {
			r.SetOrder(-1)
		} else {
			r.SetOrder(spin.Value())
		}
	})

	dialog.Show()
}

var cleaner = strings.NewReplacer(
	"\n", " ",
	"\n\n", "\n",
)

func clean(str string) string {
	return cleaner.Replace(strings.TrimSpace(str))
}

var moveToSectionCSS = cssutil.Applier("room-movetosection", `
	.room-movetosection label {
		margin: 4px 12px;
	}
	.room-movetosection entry {
		margin:  2px 4px;
		padding: 0px 4px;
	}
`)

func (r *Room) moveToSectionBox() gtk.Widgetter {
	header := gtk.NewLabel("Section Name")
	header.SetXAlign(0)
	header.SetAttributes(markuputil.Attrs(
		pango.NewAttrWeight(pango.WeightBold),
	))

	entry := gtk.NewEntry()
	entry.SetInputPurpose(gtk.InputPurposeFreeForm)
	entry.SetInputHints(gtk.InputHintSpellcheck | gtk.InputHintEmoji)
	entry.SetMaxLength(255 - len("u."))
	entry.Connect("activate", func() {
		text := entry.Text()
		if text != "" {
			r.section.MoveRoomToTag(r.ID, matrix.TagName("u."+string(text)))
		}
	})

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(header)
	box.Append(entry)
	moveToSectionCSS(box)

	return box
}
