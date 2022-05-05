package emojiview

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/components/uploadutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/emojis"
	"github.com/diamondburned/gotrix/matrix"
)

type emoji struct {
	*gtk.ListBoxRow
	emoji *gtk.Image
	name  *gtk.Label

	// states
	mxc matrix.URL
}

var emojiCSS = cssutil.Applier("emojiview-emoji", `
	.emojiview-emoji {
		padding: 8px;
	}
	.emojiview-emoji > *:not(:first-child) {
		margin-left: 8px;
	}
`)

func newEmptyEmoji(name emojis.EmojiName) emoji {
	img := gtk.NewImage()
	img.SetSizeRequest(EmojiSize, EmojiSize)

	label := gtk.NewLabel(string(name))
	label.SetXAlign(0)
	label.SetHExpand(true)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetTooltipText(string(name))

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(img)
	box.Append(label)
	emojiCSS(box)

	row := gtk.NewListBoxRow()
	row.SetName(string(name))
	row.SetChild(box)

	return emoji{
		ListBoxRow: row,
		emoji:      img,
		name:       label,
	}
}

func (e *emoji) Rename(name emojis.EmojiName) {
	e.name.SetLabel(string(name))
	e.name.SetTooltipText(string(name))
	e.SetName(string(name))
}

type uploadingEmoji struct {
	*gtk.ListBoxRow
	img    *gtk.Image
	name   *gtk.Label
	pbar   *uploadutil.ProgressBar
	action *gtk.Button
}

func newUploadingEmoji(name emojis.EmojiName) uploadingEmoji {
	img := gtk.NewImage()
	img.SetSizeRequest(EmojiSize, EmojiSize)

	label := gtk.NewLabel(string(name))
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetTooltipText(string(name))
	label.SetXAlign(0)

	prog := uploadutil.NewProgressBar()

	progLabel := gtk.NewBox(gtk.OrientationVertical, 0)
	progLabel.SetHExpand(true)
	progLabel.Append(label)
	progLabel.Append(prog)

	action := gtk.NewButtonFromIconName("process-stop-symbolic")
	action.SetHasFrame(false)
	action.SetTooltipText("Stop")

	box := gtk.NewBox(gtk.OrientationHorizontal, 5)
	box.Append(img)
	box.Append(progLabel)
	box.Append(action)
	emojiCSS(box)

	row := gtk.NewListBoxRow()
	row.SetName(string(name))
	row.SetChild(box)

	return uploadingEmoji{
		ListBoxRow: row,
		img:        img,
		name:       label,
		pbar:       prog,
		action:     action,
	}
}

type renamingEmoji struct {
	*gtk.Box
	entry *gtk.Entry
	name  emojis.EmojiName
}

func newEmojiRenameRow(name emojis.EmojiName, emoji emoji) renamingEmoji {
	img := gtk.NewImage()
	img.SetSizeRequest(EmojiSize, EmojiSize)
	img.SetFromPaintable(emoji.emoji.Paintable())

	entry := gtk.NewEntry()
	entry.SetHExpand(true)
	entry.SetText(name.Name())
	entry.SetInputPurpose(gtk.InputPurposeAlpha)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(img)
	box.Append(entry)
	emojiCSS(box)

	return renamingEmoji{
		Box:   box,
		entry: entry,
		name:  name,
	}
}
