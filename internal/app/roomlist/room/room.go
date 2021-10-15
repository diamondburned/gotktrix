package room

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/components/dialogs"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/locale"
	"github.com/pkg/errors"

	localemsg "golang.org/x/text/message"
)

// AvatarSize is the size in pixels of the avatar.
const AvatarSize = 32

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

	isUnread    bool
	showPreview bool
}

var rowCSS = cssutil.Applier("room-row", `
	.room-row:selected {
		background: inherit;
	}
	.room-row:hover,
	.room-row:focus {
		background: alpha(@theme_fg_color, .15);
	}
	.room-row.room-active {
		background-color: alpha(@accent_color, 0.45);
	}
	.room-row.room-active:hover,
	.room-row.room-active:focus {
		background: alpha(mix(@theme_fg_color, @accent_bg_color, 0.5), .6);
	}
`)

var avatarCSS = cssutil.Applier("room-avatar", ``)

var roomBoxCSS = cssutil.Applier("room-box", `
	.room-box {
		padding:  2px 6px;
		padding-left: 4px;
		border-left:  2px solid transparent;
	}
	.room-unread .room-box {
		border-left:  2px solid @theme_fg_color;
	}
	.room-right {
		margin-left: 6px;
	}
	.room-preview {
		font-size: 0.8em;
	}
`)

// Section is the controller interface that Room holds as its parent section.
type Section interface {
	Tag() matrix.TagName
	InvalidateSort()

	Remove(*Room)
	Insert(*Room)

	OpenRoom(matrix.RoomID)
	OpenRoomInTab(matrix.RoomID)

	// MoveRoomToTag moves the room with the given ID to the given tag name. A
	// new section must be created if needed.
	MoveRoomToTag(src matrix.RoomID, tag matrix.TagName) bool
}

// roomEvents is the list of room state events to subscribe to.
var roomEvents = []event.Type{
	event.TypeRoomName,
	event.TypeRoomCanonicalAlias,
	event.TypeRoomAvatar,
	m.FullyReadEventType,
}

// AddTo adds an empty room with the given ID to the given section Rooms created
// using this constructor will automatically update itself as soon as it's added
// into a parent, so the caller does not have to trigger the Invalidate methods.
func AddTo(ctx context.Context, section Section, roomID matrix.RoomID) *Room {
	nameLabel := gtk.NewLabel(string(roomID))
	nameLabel.SetSingleLineMode(true)
	nameLabel.SetXAlign(0)
	nameLabel.SetHExpand(true)
	nameLabel.SetEllipsize(pango.EllipsizeEnd)
	nameLabel.AddCSSClass("room-name")

	previewLabel := gtk.NewLabel("")
	previewLabel.SetSingleLineMode(true)
	previewLabel.SetXAlign(0)
	previewLabel.SetHExpand(true)
	previewLabel.SetEllipsize(pango.EllipsizeEnd)
	previewLabel.AddCSSClass("room-preview")
	previewLabel.Hide()

	rightBox := gtk.NewBox(gtk.OrientationVertical, 0)
	rightBox.SetVAlign(gtk.AlignCenter)
	rightBox.Append(nameLabel)
	rightBox.Append(previewLabel)
	rightBox.AddCSSClass("room-right")

	adwAvatar := adw.NewAvatar(AvatarSize, string(roomID), false)
	avatarCSS(&adwAvatar.Widget)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(&adwAvatar.Widget)
	box.Append(rightBox)
	roomBoxCSS(box)

	row := gtk.NewListBoxRow()
	row.SetChild(box)
	row.SetName(string(roomID))
	rowCSS(row)

	r := Room{
		ListBoxRow: row,
		box:        box,
		name:       nameLabel,
		preview:    previewLabel,
		avatar:     adwAvatar,

		ctx:     gtkutil.WithVisibility(ctx, row),
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
		"add-emojis":      func() { emojiview.ForRoom(r.ctx, r.ID) },
	})

	gtkutil.BindRightClick(row, func() {
		s := locale.SFunc(ctx)

		p := gtkutil.PopoverMenuCustom(row, gtk.PosBottom, []gtkutil.PopoverMenuItem{
			gtkutil.MenuItem(s("Open"), "room.open"),
			gtkutil.MenuItem(s("Open in New Tab"), "room.open-in-tab"),
			gtkutil.MenuSeparator(s("Section")),
			gtkutil.MenuItem(s("Reorder Room..."), "room.prompt-reorder"),
			gtkutil.Submenu(s("Move to Section..."), []gtkutil.PopoverMenuItem{
				gtkutil.MenuWidget("room.move-to-section", r.moveToSectionBox()),
			}),
			gtkutil.MenuSeparator(s("Emojis")),
			gtkutil.MenuItem(s("Add Emojis..."), "room.add-emojis"),
		})
		p.SetAutohide(true)
		p.SetCascadePopdown(true)
		p.Popup()
	})

	client := gotktrix.FromContext(r.ctx).Offline()

	// Bind the message handler to update itself.
	gtkutil.MapSubscriber(row, func() func() {
		r.InvalidateRead()
		r.InvalidateName()
		r.InvalidateAvatar()
		r.InvalidatePreview()

		f1 := client.SubscribeTimeline(roomID, func(event.RoomEvent) {
			glib.IdleAdd(func() {
				r.InvalidatePreview()
				r.setUnread(true)
				r.section.InvalidateSort()
			})
		})

		f2 := client.SubscribeRoomEvents(roomID, roomEvents, func(ev event.Event) {
			glib.IdleAdd(func() {
				switch ev.(type) {
				case event.RoomNameEvent, event.RoomCanonicalAliasEvent:
					r.InvalidateName()
				case event.RoomAvatarEvent:
					r.InvalidateAvatar()
				case m.FullyReadEvent:
					r.InvalidateRead()
					r.section.InvalidateSort()
				}
			})
		})

		return func() {
			f1()
			f2()
		}
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

// SetActive sets whether or not the room is active. This is different from
// being selected, since keyboard shortcuts may select a room but not activate
// it.
func (r *Room) SetActive(active bool) {
	if active {
		if !r.HasCSSClass("room-active") {
			r.AddCSSClass("room-active")
		}
	} else {
		r.RemoveCSSClass("room-active")
	}
}

// InvalidateName invalidates the room's name and refetches them from the state
// or API.
func (r *Room) InvalidateName() {
	client := gotktrix.FromContext(r.ctx)

	n, err := client.Offline().RoomName(r.ID)
	if err == nil && n != "Empty Room" {
		r.setLabel(n)
		return
	}

	// Goroutines are cheap as hell!
	go func() {
		n, err := client.RoomName(r.ID)
		if err == nil {
			glib.IdleAdd(func() { r.setLabel(n) })
		}
	}()
}

// InvalidateAvatar invalidates the room's avatar.
func (r *Room) InvalidateAvatar() {
	client := gotktrix.FromContext(r.ctx)
	ctx := r.ctx.Take()

	go func() {
		mxc, _ := client.RoomAvatar(r.ID)
		if mxc == nil {
			return
		}

		url, _ := client.SquareThumbnail(*mxc, AvatarSize)

		p, err := imgutil.GET(ctx, url)
		if err == nil {
			glib.IdleAdd(func() { r.avatar.SetCustomImage(p) })
		}
	}()
}

// setLabel sets the room name.
func (r *Room) setLabel(text string) {
	r.Name = text
	r.name.SetLabel(text)
	r.name.SetTooltipText(text)
	r.avatar.SetName(text)
	r.avatar.SetTooltipText(text)
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

// InvalidatePreview invalidate the room's preview. It only queries the state.
func (r *Room) InvalidatePreview() {
	if !r.showPreview {
		r.erasePreview()
		return
	}

	client := gotktrix.FromContext(r.ctx).Offline()

	var foundEv *gotktrix.EventBox
	var firstEv *gotktrix.EventBox
	extra := -1

	err := client.EachTimelineReverse(r.ID, func(ev *gotktrix.EventBox) error {
		extra++

		isMsg := ev.Type == event.TypeRoomMessage
		if !isMsg && firstEv != nil {
			return nil
		}

		if firstEv == nil {
			firstEv = ev
		}

		if isMsg {
			foundEv = ev
			return gotktrix.EachBreak
		}

		return nil
	})

	if foundEv == nil {
		foundEv = firstEv
		extra = 0
	}

	if err != nil {
		log.Println("failed to fetch timeline for preview:", err)
	}

	if foundEv == nil {
		r.erasePreview()
		return
	}

	preview := message.RenderEvent(r.ctx, foundEv)

	if extra == 0 {
		r.preview.SetMarkup(preview)
	} else {
		r.preview.SetMarkup(fmt.Sprintf(
			`<span alpha="75%%" size="small">(+%d)</span> %s`,
			extra, preview,
		))
	}

	r.preview.SetTooltipMarkup(preview)
	r.preview.Show()
}

// InvalidateRead invalidates the read state of this room.
func (r *Room) InvalidateRead() {
	client := gotktrix.FromContext(r.ctx)

	if unread, ok := client.Offline().RoomIsUnread(r.ID); ok {
		r.setUnread(unread)
		return
	}

	go func() {
		unread, ok := client.RoomIsUnread(r.ID)
		if ok {
			glib.IdleAdd(func() { r.setUnread(unread) })
		}
	}()
}

func (r *Room) setUnread(unread bool) {
	// If the room is currently selected, then don't mark it as unread.
	if unread && r.IsSelected() {
		unread = false
	}

	if r.isUnread == unread {
		return
	}

	r.isUnread = unread

	if unread {
		r.AddCSSClass("room-unread")
	} else {
		r.RemoveCSSClass("room-unread")
	}
}

// IsUnread returns true if the room is currently not read. If the room is not
// yet mapped, then it'll always be false. The room will invoke InvalidateSort
// on its parent section if this boolean changes.
func (r *Room) IsUnread() bool {
	return r.isUnread
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
	msg := locale.S(r.ctx, localemsg.Key("reorder-help", reorderHelp))

	help := gtk.NewLabel(clean(msg))
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

	dialog := dialogs.New(
		app.Window(r.ctx),
		locale.Sprint(r.ctx, "Discard"),
		locale.Sprint(r.ctx, "Save"),
	)
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
	header := gtk.NewLabel(locale.Sprint(r.ctx, "Section Name"))
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
