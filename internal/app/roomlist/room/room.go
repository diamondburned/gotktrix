package room

import (
	"context"
	"fmt"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/emojiview"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/components/dialogs"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
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

	avatar *adaptive.Avatar
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

	ID        matrix.RoomID
	Name      string
	AvatarURL matrix.URL

	ctx     gtkutil.Cancellable
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
	.room-unread .room-box {
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

// roomEvents is the list of room state events to subscribe to.
var roomEvents = []event.Type{
	event.TypeRoomName,
	event.TypeRoomCanonicalAlias,
	event.TypeRoomAvatar,
	m.FullyReadEventType,
}

var showMessagePreview = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Message Preview",
	Section:     "Appearance",
	Description: "Show part of the latest message for each room.",
})

// AddTo adds an empty room with the given ID to the given section Rooms created
// using this constructor will automatically update itself as soon as it's added
// into a parent, so the caller does not have to trigger the Invalidate methods.
func AddTo(ctx context.Context, section Section, roomID matrix.RoomID) *Room {
	r := Room{
		section: section,
		ID:      roomID,
		Name:    string(roomID),
	}

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

	r.avatar = adaptive.NewAvatar(AvatarSize)
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

	section.Insert(&r)

	gtkutil.BindActionMap(r, "room", map[string]func(){
		"open":            func() { section.OpenRoom(roomID) },
		"open-in-tab":     func() { section.OpenRoomInTab(roomID) },
		"prompt-reorder":  func() { r.promptReorder() },
		"move-to-section": nil,
		"add-emojis":      func() { emojiview.ForRoom(r.ctx.Take(), r.ID) },
	})

	gtkutil.BindRightClick(r, func() {
		s := locale.SFunc(ctx)

		p := gtkutil.PopoverMenuCustom(r, gtk.PosBottom, []gtkutil.PopoverMenuItem{
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

	client := gotktrix.FromContext(r.ctx.Take()).Offline()

	// Bind the message handler to update itself.
	r.ctx.OnRenew(func(ctx context.Context) func() {
		r.InvalidateName(ctx)
		r.InvalidateAvatar(ctx)
		r.InvalidatePreview(ctx)

		b := gtkutil.FuncBatcher()
		b.F(client.SubscribeTimelineSync(roomID, func(event.RoomEvent) {
			gtkutil.IdleCtx(ctx, func() {
				r.InvalidatePreview(ctx)
				r.Changed()
			})
		}))
		b.F(client.SubscribeRoomEvents(roomID, roomEvents, func(ev event.Event) {
			gtkutil.IdleCtx(ctx, func() {
				switch ev.(type) {
				case *event.RoomNameEvent, *event.RoomCanonicalAliasEvent:
					r.InvalidateName(ctx)
					r.Changed()
				case *event.RoomAvatarEvent:
					r.InvalidateAvatar(ctx)
				case *m.FullyReadEvent:
					r.InvalidatePreview(ctx)
					r.Changed()
				}
			})
		}))

		return b.Done()
	})

	// Initialize drag-and-drop.
	drag := gtkutil.NewDragSourceWithContent(r, gdk.ActionMove, string(roomID))
	r.AddController(drag)

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

// InvalidateName invalidates the room's name and refetches them from the state
// or API.
func (r *Room) InvalidateName(ctx context.Context) {
	client := gotktrix.FromContext(ctx)

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
func (r *Room) InvalidateAvatar(ctx context.Context) {
	client := gotktrix.FromContext(ctx)

	mxc, err := client.Offline().RoomAvatar(r.ID)
	if err == nil {
		r.setAvatarURL(mxc)
		if mxc != nil {
			url, _ := client.SquareThumbnail(*mxc, AvatarSize, gtkutil.ScaleFactor())
			imgutil.AsyncGET(ctx, url, r.avatar.SetFromPaintable)
		}
	}

	go func() {
		mxc, _ := client.RoomAvatar(r.ID)
		glib.IdleAdd(func() { r.setAvatarURL(mxc) })

		if mxc != nil {
			url, _ := client.SquareThumbnail(*mxc, AvatarSize, gtkutil.ScaleFactor())
			imgutil.GET(ctx, url, r.avatar.SetFromPaintable)
		}
	}()
}

func (r *Room) setAvatarURL(mxc *matrix.URL) {
	if mxc == nil {
		r.AvatarURL = matrix.URL("")
		r.avatar.SetFromPaintable(nil)
		return
	}
	if r.AvatarURL == *mxc {
		return
	}
	r.AvatarURL = *mxc
}

// setLabel sets the room name.
func (r *Room) setLabel(text string) {
	r.Name = text
	r.name.label.SetLabel(text)
	r.name.label.SetTooltipText(text)
	r.avatar.SetName(text)
	r.avatar.SetTooltipText(text)
}

// SetShowMessagePreview sets whether or not the room should show the message
// preview.
func (r *Room) SetShowMessagePreview(show bool) {
	r.showPreview = show
	r.InvalidatePreview(r.ctx.Take())
}

func (r *Room) erasePreview() {
	r.preview.label.SetLabel("")
	r.preview.extra.SetLabel("")
	r.preview.Hide()
}

// InvalidatePreview invalidate the room's preview. It only queries the state.
func (r *Room) InvalidatePreview(ctx context.Context) {
	if !r.showPreview {
		r.erasePreview()
		return
	}

	// Do this in a goroutine, since it might freeze up the UI thread trying to
	// unmarshal a bunch of messages. This might make things arrive out of
	// order, but honestly, whatever.
	gtkutil.Async(ctx, func() func() {
		client := gotktrix.FromContext(ctx)

		first, extra := client.State.LatestInTimeline(r.ID, event.TypeRoomMessage)
		if first == nil {
			first, extra = client.State.LatestInTimeline(r.ID, "")
		}
		if first == nil {
			return func() { r.erasePreview() }
		}

		preview := message.RenderEvent(ctx, first)
		count := countUnreadFmt(client, r.ID)

		return func() {
			r.setUnread(count != "")
			r.name.unread.SetText(count)

			r.preview.label.SetMarkup(preview)
			r.preview.label.SetTooltipMarkup(preview)
			r.preview.Show()

			if extra > 0 {
				r.preview.extra.SetLabel(fmt.Sprintf("+%d events", extra))
			} else {
				r.preview.extra.SetLabel("")
			}
		}
	})
}

func countUnreadFmt(client *gotktrix.Client, roomID matrix.RoomID) string {
	latestID := client.RoomLatestReadEvent(roomID)
	if latestID == "" {
		return ""
	}

	var unread int
	var found bool

	client.EachTimelineReverse(roomID, func(ev event.RoomEvent) error {
		if ev.RoomInfo().ID == latestID {
			found = true
			return gotktrix.EachBreak
		}
		unread++
		return nil
	})

	if unread == 0 {
		return ""
	}

	var s string
	if found {
		s = fmt.Sprintf("(%d)", unread)
	} else {
		s = fmt.Sprintf("(%d+)", unread)
	}

	return s
}

func (r *Room) setUnread(unread bool) {
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
		app.Window(ctx),
		locale.S(ctx, "Discard"),
		locale.S(ctx, "Save"),
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
	header := gtk.NewLabel(locale.Sprint(r.ctx.Take(), "Section Name"))
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
