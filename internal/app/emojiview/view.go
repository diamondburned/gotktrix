package emojiview

import (
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
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/pkg/errors"
)

// EmojiSize is the size of each emoji in widget size.
const EmojiSize = 32

type View struct {
	*adw.Clamp
	list *gtk.ListBox
	name *gtk.Label

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
	.emojiview-box .emojiview-actions {
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

	buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	buttonBox.SetCSSClasses([]string{"linked", "emojiview-actions"})
	buttonBox.Append(delButton)
	buttonBox.Append(addButton)

	topBox := gtk.NewBox(gtk.OrientationHorizontal, 4)
	topBox.Append(boxLabel)
	topBox.Append(buttonBox)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(topBox)
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

		emojis: map[emojis.EmojiName]emoji{},
		roomID: roomID,

		app:    app,
		client: app.Client,
	}

	view.InvalidateName()
	view.Invalidate()

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
		row := newEmptyEmoji(name)
		row.mxc = emoji.URL

		url, _ := v.client.SquareThumbnail(row.mxc, EmojiSize*2)
		imgutil.AsyncGET(v.stop.Context(), url, row.emoji.SetFromPaintable)

		v.list.Insert(row, -1)
		v.emojis[name] = row
	}
}
