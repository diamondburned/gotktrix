package emojiview

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/actionbutton"
	"github.com/diamondburned/gotkit/components/dialogs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/components/uploadutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/emojis"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

// EmojiSize is the size of each emoji in widget size.
const EmojiSize = 32

type View struct {
	*gtk.Box
	list *gtk.ListBox
	name *gtk.Label
	sync *gtk.Button

	search string
	emojis map[emojis.EmojiName]emoji
	roomID matrix.RoomID // empty if user, constant

	ctx    gtkutil.Canceller
	client *gotktrix.Client
}

var boxCSS = cssutil.Applier("emojiview-box", `
	.emojiview-box {
		padding: 8px;
	}
	.emojiview-box .emojiview-name {
		margin: 0 8px;
	}
	.emojiview-box .emojiview-rightbox {
		margin-bottom: 8px;
	}
`)

var nameAttrs = textutil.Attrs(
	pango.NewAttrScale(1.2),
	pango.NewAttrWeight(pango.WeightBold),
	pango.NewAttrInsertHyphens(false),
)

func dialog(ctx context.Context, v *View) {
	dialog := gtk.NewDialog()
	dialog.SetTransientFor(app.GTKWindowFromContext(ctx))
	dialog.SetDefaultSize(400, 500)
	dialog.SetChild(v)
	dialog.SetTitle(app.FromContext(ctx).SuffixedTitle("Add Emojis for " + v.name.Label()))
	dialog.Show()
}

// ForRoom creates a new emoji view dialog for a room.
func ForRoom(ctx context.Context, roomID matrix.RoomID) *View {
	v := new(ctx, roomID)
	dialog(ctx, v)
	return v
}

// ForUser creates a new emoji view dialog for the current user.
func ForUser(ctx context.Context) *View {
	v := new(ctx, "")
	dialog(ctx, v)
	return v
}

func new(ctx context.Context, roomID matrix.RoomID) *View {
	list := gtk.NewListBox()
	list.SetShowSeparators(true)
	list.SetSelectionMode(gtk.SelectionMultiple)
	list.SetActivateOnSingleClick(false)
	list.SetPlaceholder(gtk.NewLabel("No emojis yet..."))
	list.SetSortFunc(func(r1, r2 *gtk.ListBoxRow) int {
		return sortutil.CmpFold(r1.Name(), r2.Name())
	})

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(list)

	busy := gtk.NewSpinner()
	busy.Stop()
	busy.Hide()

	renameButton := newActionButton("Rename", "document-edit-symbolic")
	renameButton.SetSensitive(false)

	delButton := newActionButton("Remove", "list-remove-symbolic")
	delButton.SetSensitive(false)
	addButton := newActionButton("Add", "list-add-symbolic")
	actionBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	actionBox.SetCSSClasses([]string{"linked"})
	actionBox.Append(delButton)
	actionBox.Append(addButton)

	syncButton := newFullActionButton("Sync", "emblem-synchronizing-symbolic")
	syncButton.SetSensitive(false)

	rightBox := gtk.NewBox(gtk.OrientationHorizontal, 5)
	rightBox.SetCSSClasses([]string{"emojiview-rightbox"})
	rightBox.SetHAlign(gtk.AlignEnd)
	rightBox.Append(busy)
	rightBox.Append(renameButton)
	rightBox.Append(actionBox)
	rightBox.Append(syncButton)

	boxLabel := gtk.NewLabel(string(roomID))
	boxLabel.SetCSSClasses([]string{"emojiview-name"})
	boxLabel.SetHExpand(true)
	boxLabel.SetWrap(true)
	boxLabel.SetWrapMode(pango.WrapWordChar)
	boxLabel.SetXAlign(0)
	boxLabel.SetAttributes(nameAttrs)

	// Use a leaflet here and make it behave like a box.
	top := gtk.NewFlowBox()
	top.Insert(boxLabel, -1)
	top.Insert(rightBox, -1)
	top.SetActivateOnSingleClick(false)
	top.SetSelectionMode(gtk.SelectionNone)
	top.SetColumnSpacing(5)
	top.SetMinChildrenPerLine(1)
	top.SetMaxChildrenPerLine(2)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(top)
	box.Append(scroll)
	boxCSS(box)

	list.ConnectSelectedRowsChanged(func() {
		// Allow pressing the delete button if we have selected rows.
		selected := len(list.SelectedRows()) > 0
		delButton.SetSensitive(selected)
		renameButton.SetSensitive(selected)
	})

	view := &View{
		Box:  box,
		list: list,
		name: boxLabel,
		sync: syncButton,

		emojis: map[emojis.EmojiName]emoji{},
		roomID: roomID,

		ctx:    gtkutil.WithCanceller(ctx),
		client: gotktrix.FromContext(ctx),
	}

	view.InvalidateName()
	view.Invalidate()

	list.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
		return view.search == "" || strings.Contains(row.Name(), view.search)
	})

	renameButton.ConnectClicked(func() {
		selected := list.SelectedRows()
		names := make([]emojis.EmojiName, len(selected))

		for i, row := range selected {
			names[i] = emojis.EmojiName(row.Name())
		}

		view.promptRenameEmojis(names)
	})

	addButton.ConnectClicked(func() {
		chooser := newFileChooser(ctx, view.addEmotesFromFiles)
		chooser.Show()
	})

	delButton.ConnectClicked(func() {
		for _, row := range list.SelectedRows() {
			delete(view.emojis, emojis.EmojiName(row.Name()))
			list.Remove(row)
		}

		syncButton.SetSensitive(true)
	})

	syncButton.ConnectClicked(func() {
		syncButton.SetSensitive(false)
		busy.Start()
		busy.Show()

		view.syncEmojis(busy)
	})

	return view
}

func newActionButton(name, icon string) *gtk.Button {
	button := gtk.NewButtonFromIconName(icon)
	button.SetTooltipText(name)

	return button
}

func newFullActionButton(name, icon string) *gtk.Button {
	btn := actionbutton.NewButton(name, icon, gtk.PosLeft)
	return btn.Button
}

// Stop cancels the background context, which cancels any background jobs.
func (v *View) Stop() {
	v.ctx.Cancel()
}

// InvalidateName invalidates the name at the top left corner.
func (v *View) InvalidateName() {
	var name string

	// TODO: async
	client := v.client.Offline()

	if v.roomID != "" {
		name, _ = v.client.Offline().RoomName(v.roomID)
	} else {
		// Use username.
		id, err := client.Whoami()
		if err == nil {
			name, _, _ = id.Parse()
		}
	}

	v.name.SetLabel(name)
}

// Invalidate invalidates the emoji list and re-renders everything. If the given
// room ID is empty, then the user's emojis are fetched.
func (v *View) Invalidate() {
	v.ctx.Renew()

	e, err := fetchEmotes(v.client.Offline(), v.roomID)
	if err != nil {
		v.onlineFetch()
		return
	}

	v.useEmoticonEvent(e)
}

func (v *View) onlineFetch() {
	ctx := v.ctx.Take()
	client := v.client.WithContext(ctx)

	go func() {
		e, err := fetchEmotes(client, v.roomID)
		if err != nil {
			return
		}

		gtkutil.IdleCtx(ctx, func() { v.useEmoticonEvent(e) })
	}()
}

func fetchEmotes(client *gotktrix.Client, roomID matrix.RoomID) (emojis.EmojiMap, error) {
	if roomID != "" {
		return emojis.RoomEmotes(client, roomID)
	} else {
		return emojis.UserEmotes(client)
	}
}

// ToData converts View's internal state to an EmoticonEventData type.
func (v *View) ToData() emojis.EmoticonEventData {
	emoticons := make(map[emojis.EmojiName]emojis.Emoji, len(v.emojis))

	for name, emoji := range v.emojis {
		emoticons[name] = emojis.Emoji{
			URL: emoji.mxc,
		}
	}

	return emojis.EmoticonEventData{
		Emoticons: emoticons,
	}
}

func (v *View) syncEmojis(busy *gtk.Spinner) {
	ctx := v.ctx.Take()
	client := v.client.WithContext(ctx)

	ev := v.ToData()

	go func() {
		defer glib.IdleAdd(func() {
			busy.Stop()
			busy.Hide()
			v.list.UnselectAll()
		})

		var err error
		if v.roomID != "" {
			err = client.ClientConfigRoomSet(v.roomID, string(emojis.RoomEmotesEventType), ev)
		} else {
			err = client.ClientConfigSet(string(emojis.UserEmotesEventType), ev)
		}

		if err != nil {
			app.Error(ctx, errors.Wrap(err, "failed to set emojis config"))
			return
		}
	}()
}

func (v *View) promptRenameEmojis(names []emojis.EmojiName) {
	listBox := gtk.NewBox(gtk.OrientationVertical, 2)
	renames := make([]renamingEmoji, 0, len(names))

	for _, name := range names {
		if emoji, ok := v.emojis[name]; ok {
			w := newEmojiRenameRow(name, emoji)

			renames = append(renames, w)
			listBox.Append(w)
		}
	}

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(listBox)

	dialog := dialogs.NewLocalize(v.ctx.Take(), "Cancel", "Save")
	dialog.SetDefaultSize(300, 240)
	dialog.SetTitle("Rename Emojis")
	dialog.SetChild(scroll)
	dialog.Show()

	dialog.Cancel.ConnectClicked(dialog.Close)
	dialog.OK.ConnectClicked(func() {
		for _, rename := range renames {
			v.renameEmoji(rename.name, newEmojiName(rename.entry.Text()))
		}

		v.list.InvalidateSort()
		v.sync.SetSensitive(true)
		dialog.Close()
	})
}

// renameEmoji does NOT update UI state, except for the row name.
func (v *View) renameEmoji(old, new emojis.EmojiName) {
	if old == new {
		return
	}

	emoji := v.emojis[old]
	emoji.Rename(new)

	delete(v.emojis, old)
	v.emojis[new] = emoji
}

func (v *View) useEmoticonEvent(emojiMap emojis.EmojiMap) {
	// Check for existing emojis.
	for name, emoji := range emojiMap {
		old, ok := v.emojis[name]
		if !ok {
			// Emoji does not exist; fetch it later.
			continue
		}

		if old.mxc == emoji.URL {
			// Same URL. Skip.
			continue
		}

		// Emoji of the same name exists but with a different URL. Update the
		// avatar and move on.

		old.mxc = emoji.URL
		v.emojis[name] = old

		url, _ := v.client.SquareThumbnail(old.mxc, EmojiSize, gtkutil.ScaleFactor())
		imgutil.AsyncGET(v.ctx.Take(), url, imgutil.ImageSetterFromImage(old.emoji))

		delete(emojiMap, name)
		continue
	}

	// Remove deleted emojis.
	for name, emoji := range v.emojis {
		_, ok := v.emojis[name]
		if ok {
			continue
		}

		v.list.Remove(emoji)
		delete(v.emojis, name)
	}

	// Add missing emojis.
	for name, emoji := range emojiMap {
		v.addEmoji(name, emoji.URL)
	}
}

func (v *View) addEmoji(name emojis.EmojiName, mxc matrix.URL) emoji {
	emoji := newEmptyEmoji(name)
	emoji.mxc = mxc

	url, _ := v.client.SquareThumbnail(emoji.mxc, EmojiSize, gtkutil.ScaleFactor())
	imgutil.AsyncGET(v.ctx.Take(), url, imgutil.ImageSetterFromImage(emoji.emoji))

	v.list.Insert(emoji, -1)
	v.emojis[name] = emoji

	return emoji
}

func (v *View) addEmotesFromFiles(paths []string) {
	// Create pseudo-emojis.
	for _, path := range paths {
		v.addEmotesFromfile(path)
	}
}

func (v *View) addEmotesFromfile(path string) {
	name := emojiNameFromFile(path)

	emoji := newUploadingEmoji(name)
	emoji.img.SetFromFile(path)

	ctx, cancel := context.WithCancel(v.ctx.Take())

	emoji.action.ConnectClicked(func() {
		emoji.action.SetSensitive(false)
		cancel()
	})

	v.list.Append(emoji)

	onError := func(err error) {
		prefix := strings.Trim(string(name), ":")
		emoji.name.SetMarkup(textutil.ErrorMarkup(prefix + ": " + err.Error()))
		emoji.pbar.Error()

		emoji.action.SetIconName("view-refresh-symbolic")
		emoji.action.SetSensitive(false)
	}

	go func() {
		defer cancel()

		f, err := os.Open(path)
		if err != nil {
			glib.IdleAdd(func() { onError(err) })
			return
		}
		defer f.Close()

		if s, _ := f.Stat(); s != nil {
			emoji.pbar.SetTotal(s.Size())
		}

		r := uploadutil.WrapProgressReader(emoji.pbar, f)
		defer r.Close()

		u, err := uploadutil.Upload(v.client.WithContext(ctx), r, filepath.Base(path))
		if err != nil {
			glib.IdleAdd(func() { onError(err) })
			return
		}

		glib.IdleAdd(func() {
			v.list.Remove(emoji)
			v.addEmoji(name, u)
			v.sync.SetSensitive(true)
		})
	}()
}

func emojiNameFromFile(path string) emojis.EmojiName {
	filename := filepath.Base(path)
	parts := strings.SplitN(filename, ".", 2)
	return newEmojiName(parts[0])
}

func newEmojiName(name string) emojis.EmojiName {
	return emojis.EmojiName(":" + strings.Trim(name, ":") + ":")
}
