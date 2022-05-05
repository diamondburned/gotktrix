package room

import (
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/dialogs"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"

	localemsg "golang.org/x/text/message"
)

// AvatarSize is the size in pixels of the avatar.
const AvatarSize = 32

// Room is a single room row.
type Room struct {
	*State
	*gtk.ListBoxRow
	box *gtk.Box

	avatar *onlineimage.Avatar
	right  *gtk.Box

	name struct {
		*gtk.Box
		label  *gtk.Label
		unread *gtk.Label
	}

	preview struct {
		*gtk.Box
		label *gtk.Label
		extra *gtk.Label
	}

	ctx     gtkutil.Cancellable
	section Section
}

var rowCSS = cssutil.Applier("room-row", `
	.room-row:selected {
		background: inherit;
	}
	.room-row:hover,
	.room-row:focus {
		background: @borders;
	}
	.room-row.room-active {
		background-color: mix(@theme_selected_bg_color, @borders, 0.5); 
	}
	.room-row.room-active:hover,
	.room-row.room-active:focus {
		background: mix(mix(@theme_selected_bg_color, @borders, 0.5), @theme_fg_color, 0.2);
	}
`)

var avatarCSS = cssutil.Applier("room-avatar", ``)

var roomBoxCSS = cssutil.Applier("room-box", `
	.room-box {
		padding:  2px 6px;
		padding-left: 4px;
		padding-right: 0;
		border-left:  2px solid transparent;
	}
	.room-unread-message .room-box {
		border-left:  2px solid @theme_fg_color;
	}
	.room-right {
		margin-left: 6px;
	}
	.room-preview {
		margin-right: 2px;
	}
	.room-unread-count,
	.room-preview,
	.room-preview-extra {
		font-size: 0.8em;
	}
	.room-unread-count,
	.room-preview-extra {
		color: alpha(@theme_fg_color, 0.75);
		margin-left: 2px;
	}
	.room-highlighted-message {
		/* See message/message.go @ messageCSS. */
		background-color: alpha(@highlighted_message, 0.15);
	}
	.room-highlighted-message:hover,
	.room-highlighted-message:focus {
		background: mix(alpha(@highlighted_message, 0.15), @theme_fg_color, 0.15);
	}
	.room-highlighted-message .room-box {
		border-color: @highlighted_message;
	}
`)

// Section is the controller interface that Room holds as its parent section.
type Section interface {
	Tag() matrix.TagName

	Changed(*Room)
	Remove(*Room)
	Insert(*Room)

	OpenRoom(matrix.RoomID)
	OpenRoomInTab(matrix.RoomID)

	// MoveRoomToTag moves the room with the given ID to the given tag name. A
	// new section must be created if needed.
	MoveRoomToTag(src matrix.RoomID, tag matrix.TagName) bool
}

var showMessagePreview = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Message Preview",
	Section:     "Rooms",
	Description: "Show part of the latest message for each room.",
})

var showEventNum = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Count Events",
	Section:     "Rooms",
	Description: "Show the number of events leading up to a message in a room.",
})

// AddTo adds an empty room with the given ID to the given section Rooms created
// using this constructor will automatically update itself as soon as it's added
// into a parent, so the caller does not have to trigger the Invalidate methods.
func AddTo(ctx context.Context, section Section, roomID matrix.RoomID) *Room {
	r := Room{section: section}

	r.name.label = gtk.NewLabel(string(roomID))
	r.name.label.SetSingleLineMode(true)
	r.name.label.SetXAlign(0)
	r.name.label.SetHExpand(true)
	r.name.label.SetEllipsize(pango.EllipsizeEnd)
	r.name.label.AddCSSClass("room-name")

	r.name.unread = gtk.NewLabel("")
	r.name.unread.SetXAlign(1)
	r.name.unread.AddCSSClass("room-unread-count")

	r.name.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	r.name.Box.Append(r.name.label)
	r.name.Box.Append(r.name.unread)

	r.preview.label = gtk.NewLabel("")
	r.preview.label.AddCSSClass("room-preview")
	r.preview.label.SetSingleLineMode(true)
	r.preview.label.SetXAlign(0)
	r.preview.label.SetHExpand(true)
	r.preview.label.SetEllipsize(pango.EllipsizeEnd)

	r.preview.extra = gtk.NewLabel("")
	r.preview.extra.SetXAlign(1)
	r.preview.extra.AddCSSClass("room-preview-extra")

	r.preview.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	r.preview.Append(r.preview.label)
	r.preview.Append(r.preview.extra)

	r.right = gtk.NewBox(gtk.OrientationVertical, 0)
	r.right.AddCSSClass("room-right")
	r.right.SetVAlign(gtk.AlignCenter)
	r.right.Append(r.name)
	r.right.Append(r.preview)

	r.avatar = onlineimage.NewAvatar(ctx, gotktrix.AvatarProvider, AvatarSize)
	r.avatar.ConnectLabel(r.name.label)
	avatarCSS(r.avatar)

	r.box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	r.box.Append(r.avatar)
	r.box.Append(r.right)
	roomBoxCSS(r.box)

	r.ListBoxRow = gtk.NewListBoxRow()
	r.ListBoxRow.SetChild(r.box)
	r.ListBoxRow.SetName(string(roomID))
	rowCSS(r.ListBoxRow)

	r.ctx = gtkutil.WithVisibility(ctx, r)

	updateName := func(s State) {
		r.name.label.SetLabel(s.Name)
		if s.Topic != "" {
			r.name.label.SetTooltipText(s.Name + "\n" + s.Topic)
		} else {
			r.name.label.SetTooltipText(s.Name)
		}
	}

	r.State = NewState(ctx, roomID)
	r.State.NotifyName(func(ctx context.Context, s State) {
		r.avatar.SetName(s.Name)
		r.avatar.SetTooltipText(s.Name)
		updateName(s)
	})
	r.State.NotifyTopic(func(ctx context.Context, s State) {
		updateName(s)
	})
	r.State.NotifyAvatar(func(ctx context.Context, s State) {
		r.avatar.SetFromURL(string(s.Avatar))
	})

	section.Insert(&r)

	gtkutil.BindActionMap(r, map[string]func(){
		"room.open":            func() { section.OpenRoom(roomID) },
		"room.open-in-tab":     func() { section.OpenRoomInTab(roomID) },
		"room.prompt-reorder":  func() { r.promptReorder() },
		"room.move-to-section": nil,
		"room.add-emojis":      func() { emojiview.ForRoom(r.ctx.Take(), r.ID) },
	})

	gtkutil.BindRightClick(r, func() {
		s := locale.SFunc(ctx)

		p := gtkutil.NewPopoverMenuCustom(r, gtk.PosBottom, []gtkutil.PopoverMenuItem{
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
		gtkutil.PopupFinally(p)
	})

	client := gotktrix.FromContext(r.ctx.Take()).Offline()

	// Bind the message handler to update itself.
	r.ctx.OnRenew(func(ctx context.Context) func() {
		r.InvalidatePreview(ctx)

		return gtkutil.FuncBatcher(
			r.State.Subscribe(),
			client.SubscribeRoomSync(roomID, func() {
				fn := r.invalidatePreview(ctx)
				gtkutil.IdleCtx(ctx, func() {
					fn()
					r.Changed()
				})
			}),
		)
	})

	// Initialize drag-and-drop.
	drag := gtkutil.NewDragSourceWithContent(r, gdk.ActionMove, string(roomID))
	r.AddController(drag)

	showEventNum.SubscribeWidget(r, func() { r.InvalidatePreview(r.ctx.Take()) })
	showMessagePreview.SubscribeWidget(r, func() { r.InvalidatePreview(r.ctx.Take()) })

	return &r
}

// Section returns the current section that the room is in.
func (r *Room) Section() Section {
	return r.section
}

// Changed invalidates the order of this room within the list.
func (r *Room) Changed() {
	r.section.Changed(r)
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

func (r *Room) erasePreview() {
	r.preview.label.SetLabel("")
	r.preview.extra.SetLabel("")
	r.preview.Hide()
}

// InvalidatePreview invalidate the room's preview. It only queries the state.
func (r *Room) InvalidatePreview(ctx context.Context) {
	if !showMessagePreview.Value() {
		r.erasePreview()
		return
	}

	// Do this in a goroutine, since it might freeze up the UI thread trying to
	// unmarshal a bunch of messages. This might make things arrive out of
	// order, but honestly, whatever.
	gtkutil.Async(ctx, func() func() {
		return r.invalidatePreview(ctx)
	})
}

// invalidatePreview is called asynchronously.
func (r *Room) invalidatePreview(ctx context.Context) func() {
	if !showMessagePreview.Value() {
		return func() { r.erasePreview() }
	}

	client := gotktrix.FromContext(ctx)

	first, extra := client.State.LatestInTimeline(r.ID, event.TypeRoomMessage)
	if first == nil {
		first, extra = client.State.LatestInTimeline(r.ID, "")
	}
	if first == nil {
		return func() { r.erasePreview() }
	}

	unread, _ := client.RoomCountUnread(r.ID)
	notifications := client.State.RoomNotificationCount(r.ID)

	return func() {
		// Only show the unread bar if we have unread messages, not unread
		// any other events. We can do this by a comparison check: if there
		// are less events than unread messages, then there's an unread
		// message, otherwise if there's more, then we have none.
		if extra < unread {
			r.AddCSSClass("room-unread-message")
		} else {
			r.RemoveCSSClass("room-unread-message")
		}

		if notifications.Notification > 0 {
			r.AddCSSClass("room-notified-message")
		} else {
			r.RemoveCSSClass("room-notified-message")
		}

		if notifications.Highlight > 0 {
			r.AddCSSClass("room-highlighted-message")
		} else {
			r.RemoveCSSClass("room-highlighted-message")
		}

		if unread == 0 {
			r.RemoveCSSClass("room-unread-events")
		} else {
			r.AddCSSClass("room-unread-events")
		}

		if notifications.Notification == 0 {
			r.name.unread.SetText("")
		} else {
			r.name.unread.SetText(fmt.Sprintf("(%d)", notifications.Notification))
		}

		preview := message.RenderEvent(ctx, first)
		r.preview.label.SetMarkup(preview)
		r.preview.label.SetTooltipMarkup(preview)
		r.preview.Show()

		showEventNum := showEventNum.Value()
		r.preview.extra.SetVisible(showEventNum)

		if showEventNum && extra > 0 {
			r.preview.extra.SetLabel(locale.Sprintf(ctx, "+%d events", extra))
		} else {
			r.preview.extra.SetLabel("")
		}
	}
}

// SetOrder sets the room's order within the section it is in. If the order is
// not within [0.0, 1.0], then it is cleared.
func (r *Room) SetOrder(order float64) {
	r.SetSensitive(false)

	ctx := r.ctx.Take()
	if ctx.Err() != nil {
		return
	}

	gtkutil.Async(ctx, func() func() {
		f := func() {
			r.SetSensitive(true)
			r.Changed()
		}

		client := gotktrix.FromContext(ctx)

		tag := matrix.Tag{}
		if order >= 0 && order <= 1 {
			tag.Order = &order
		}

		if err := client.TagAdd(r.ID, r.section.Tag(), tag); err != nil {
			app.Error(ctx, errors.Wrap(err, "failed to update tag"))
			return f
		}

		if err := client.UpdateRoomTags(r.ID); err != nil {
			app.Error(ctx, errors.Wrap(err, "failed to update tag state"))
			return f
		}

		return f
	})
}

// Order returns the current room's order number, or -1 if the room doesn't have
// one.
func (r *Room) Order() float64 {
	client := gotktrix.FromContext(r.ctx.Take()).Offline()

	e, err := client.RoomEvent(r.ID, event.TypeTag)
	if err == nil {
		t, ok := e.(*event.TagEvent).Tags[r.section.Tag()]
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

var reorderHelpAttrs = textutil.Attrs(
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
	ctx := r.ctx.Take()
	msg := locale.S(ctx, localemsg.Key("reorder-help", reorderHelp))

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

	reset.ConnectToggled(func() {
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

	dialog := dialogs.NewLocalize(ctx, "Discard", "Save")
	dialog.SetDefaultSize(500, 225)
	dialog.SetChild(box)
	dialog.SetTitle("Reorder " + r.Name)

	dialog.Cancel.ConnectClicked(func() {
		dialog.Close()
	})

	dialog.OK.ConnectClicked(func() {
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
	header := gtk.NewLabel(locale.Sprint(r.ctx.Take(), "Section Name"))
	header.SetXAlign(0)
	header.SetAttributes(textutil.Attrs(
		pango.NewAttrWeight(pango.WeightBold),
	))

	entry := gtk.NewEntry()
	entry.SetInputPurpose(gtk.InputPurposeFreeForm)
	entry.SetInputHints(gtk.InputHintSpellcheck | gtk.InputHintEmoji)
	entry.SetMaxLength(255 - len("u."))
	entry.ConnectActivate(func() {
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
