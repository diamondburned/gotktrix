package emojiview

import (
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/emojis"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
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
	.emojiview-emoji label {
		margin-left: 8px;
	}
`)

func newEmptyEmoji(name emojis.EmojiName) emoji {
	img := gtk.NewImage()
	img.SetSizeRequest(EmojiSize, EmojiSize)

	label := gtk.NewLabel(string(name))
	label.SetXAlign(0)
	label.SetHExpand(true)

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

func newActionButton(name, icon string) *gtk.Button {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(gtk.NewImageFromIconName(icon))
	box.Append(gtk.NewLabel(name))

	button := gtk.NewButton()
	button.SetChild(box)

	return button
}
