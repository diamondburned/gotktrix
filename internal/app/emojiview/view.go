package emojiview

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/emojis"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/uploadutil"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/gotk3/gotk3/glib"
	"github.com/pkg/errors"
)

// EmojiSize is the size of each emoji in widget size.
const EmojiSize = 32

type View struct {
	*adw.Clamp
	list *gtk.ListBox
	name *gtk.Label
	sync *gtk.Button

	search string
	emojis map[emojis.EmojiName]emoji
	roomID matrix.RoomID // empty if user, constant

	stop gtkutil.Canceler

	app    *app.Application
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

var nameAttrs = markuputil.Attrs(
	pango.NewAttrScale(1.2),
	pango.NewAttrWeight(pango.WeightBold),
	pango.NewAttrInsertHyphens(false),
)

// NewForRoom creates a new emoji view for a room.
func NewForRoom(app *app.Application, roomID matrix.RoomID) *View {
	return new(app, roomID)
}

// NewForUser creates a new emoji view for the current user.
func NewForUser(app *app.Application) *View {
	return new(app, "")
}

func new(app *app.Application, roomID matrix.RoomID) *View {
	list := gtk.NewListBox()
	list.SetShowSeparators(true)
	list.SetSelectionMode(gtk.SelectionMultiple)
	list.SetActivateOnSingleClick(false)
	list.SetPlaceholder(gtk.NewLabel("No emojis yet..."))
	list.SetSortFunc(func(r1, r2 *gtk.ListBoxRow) int {
		return sortutil.StrcmpFold(r1.Name(), r2.Name())
	})

	delButton := newActionButton("Remove", "list-remove-symbolic")
	delButton.SetSensitive(false)
	addButton := newActionButton("Add", "list-add-symbolic")

	boxLabel := gtk.NewLabel(string(roomID))
	boxLabel.SetCSSClasses([]string{"emojiview-name"})
	boxLabel.SetHExpand(true)
	boxLabel.SetWrap(true)
	boxLabel.SetWrapMode(pango.WrapWordChar)
	boxLabel.SetXAlign(0)
	boxLabel.SetAttributes(nameAttrs)

	busy := gtk.NewSpinner()
	busy.Stop()
	busy.Hide()

	actionBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	actionBox.SetCSSClasses([]string{"linked"})
	actionBox.Append(delButton)
	actionBox.Append(addButton)

	syncButton := newActionButton("Sync", "document-save-symbolic")
	syncButton.SetSensitive(false)

	rightBox := gtk.NewBox(gtk.OrientationHorizontal, 5)
	rightBox.SetCSSClasses([]string{"emojiview-rightbox"})
	rightBox.SetHAlign(gtk.AlignEnd)
	rightBox.Append(busy)
	rightBox.Append(actionBox)
	rightBox.Append(syncButton)

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
	box.Append(list)
	boxCSS(box)

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(600)
	clamp.SetTighteningThreshold(500)
	clamp.SetChild(box)

	list.Connect("selected-rows-changed", func(list *gtk.ListBox) {
		// Allow pressing the delete button if we have selected rows.
		rows := list.SelectedRows()
		delButton.SetSensitive(len(rows) > 0)
	})

	view := &View{
		Clamp: clamp,
		list:  list,
		name:  boxLabel,
		sync:  syncButton,

		emojis: map[emojis.EmojiName]emoji{},
		roomID: roomID,

		app:    app,
		client: app.Client,
	}

	view.InvalidateName()
	view.Invalidate()

	list.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
		return view.search == "" || strings.Contains(row.Name(), view.search)
	})

	addButton.Connect("clicked", func() {
		chooser := newFileChooser(&app.Window.Window, view.addEmotesFromFiles)
		chooser.Show()
	})

	delButton.Connect("clicked", func() {
		for _, row := range list.SelectedRows() {
			delete(view.emojis, emojis.EmojiName(row.Name()))
			list.Remove(&row)
		}

		syncButton.SetSensitive(true)
	})

	syncButton.Connect("clicked", func() {
		busy.Start()
		busy.Show()

		view.syncEmojis(busy)
	})

	return view
}

// Stop cancels the background context, which cancels any background jobs.
func (v *View) Stop() {
	v.stop.Cancel()
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
	v.stop.Renew()

	e, err := fetchEmotes(v.client.Offline(), v.roomID)
	if err != nil {
		v.onlineFetch()
		return
	}

	v.useEmoticonEvent(e)
}

func (v *View) onlineFetch() {
	ctx := v.stop.Context()
	client := v.client.WithContext(ctx)

	go func() {
		e, err := fetchEmotes(client, v.roomID)
		if err != nil {
			v.app.Error(errors.Wrap(err, "failed to fetch emotes"))
			return
		}

		gtkutil.IdleCtx(ctx, func() { v.useEmoticonEvent(e) })
	}()
}

func fetchEmotes(client *gotktrix.Client, roomID matrix.RoomID) (emojis.EmoticonEventData, error) {
	if roomID != "" {
		e, err := emojis.RoomEmotes(client, roomID)
		return e.EmoticonEventData, err
	} else {
		e, err := emojis.UserEmotes(client)
		return e.EmoticonEventData, err
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
	ctx := v.stop.Context()
	client := v.client.WithContext(ctx)

	ev := v.ToData()

	go func() {
		defer glib.IdleAdd(func() {
			busy.Stop()
			busy.Hide()
		})

		var err error
		if v.roomID != "" {
			err = client.ClientConfigRoomSet(v.roomID, string(emojis.RoomEmotesEventType), ev)
		} else {
			err = client.ClientConfigSet(string(emojis.UserEmotesEventType), ev)
		}

		if err != nil {
			v.app.Error(errors.Wrap(err, "failed to set emojis config"))
			return
		}
	}()
}

func (v *View) renameEmoji(old, new emojis.EmojiName) {
	emoji := v.emojis[old]
	emoji.Rename(new)

	delete(v.emojis, old)
	v.emojis[new] = emoji

	v.list.InvalidateSort()
	v.sync.SetSensitive(true)
}

func (v *View) useEmoticonEvent(ev emojis.EmoticonEventData) {
	// Check for existing emojis.
	for name, emoji := range ev.Emoticons {
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

		url, _ := v.client.SquareThumbnail(old.mxc, EmojiSize*2)
		imgutil.AsyncGET(v.stop.Context(), url, old.emoji.SetFromPaintable)

		delete(ev.Emoticons, name)
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
	for name, emoji := range ev.Emoticons {
		v.addEmoji(name, emoji.URL)
	}
}

func (v *View) addEmoji(name emojis.EmojiName, mxc matrix.URL) emoji {
	emoji := newEmptyEmoji(name)
	emoji.mxc = mxc

	url, _ := v.client.SquareThumbnail(emoji.mxc, EmojiSize*2)
	imgutil.AsyncGET(v.stop.Context(), url, emoji.emoji.SetFromPaintable)

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

const bufferSize = 1 << 15 // 32KB

func (v *View) addEmotesFromfile(path string) {
	name := emojiNameFromFile(path)

	emoji := newUploadingEmoji(name)
	emoji.img.SetFromFile(path)

	ctx, cancel := context.WithCancel(v.stop.Context())

	emoji.action.Connect("clicked", func(action *gtk.Button) {
		action.SetSensitive(false)
		cancel()
	})

	v.list.Append(emoji)

	onError := func(err error) {
		prefix := strings.Trim(string(name), ":")
		emoji.name.SetMarkup(markuputil.Error(prefix + ": " + err.Error()))
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

		buf := bufio.NewReaderSize(r, bufferSize)

		b, err := buf.Peek(512)
		if err != nil {
			glib.IdleAdd(func() { onError(err) })
			return
		}

		u, err := v.client.WithContext(ctx).MediaUpload(
			http.DetectContentType(b),
			filepath.Base(path),
			uploadutil.WrapCloser(buf, r),
		)
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

type bufReader struct {
	*bufio.Reader
	io.Closer
}

func emojiNameFromFile(path string) emojis.EmojiName {
	filename := filepath.Base(path)
	parts := strings.SplitN(filename, ".", 2)
	return emojis.EmojiName(":" + strings.Map(emojiNameMap, parts[0]) + ":")
}

func emojiNameMap(r rune) rune {
	if r != ':' && !unicode.IsSpace(r) {
		return r
	}
	return -1
}
