package emojiview

import (
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newFileChooser(w *gtk.Window, done func([]string)) *gtk.FileChooserNative {
	filter := gtk.NewFileFilter()
	filter.AddMIMEType("image/*")

	chooser := gtk.NewFileChooserNative(
		"Upload Emojis", w, gtk.FileChooserActionOpen, "Upload", "Cancel")
	chooser.SetSelectMultiple(true)
	chooser.AddFilter(filter)
	chooser.Connect("response", func(chooser *gtk.FileChooserNative, resp int) {
		if resp != int(gtk.ResponseAccept) {
			return
		}

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
