package emojiview

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/components/filepick"
)

func newFileChooser(ctx context.Context, done func([]string)) *filepick.FilePicker {
	filter := gtk.NewFileFilter()
	filter.AddMIMEType("image/*")

	chooser := filepick.NewLocalize(
		ctx, "Upload Emojis", gtk.FileChooserActionOpen, "Upload", "Cancel")
	chooser.SetSelectMultiple(true)
	chooser.AddFilter(filter)
	chooser.ConnectAccept(func() {
		list := chooser.Files()
		length := list.NItems()
		if length == 0 {
			return
		}

		files := make([]string, length)
		for i := range files {
			f := gio.File{Object: list.Item(uint(i))}
			files[i] = f.Path()
		}

		done(files)
	})

	return chooser
}
