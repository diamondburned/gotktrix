// Package compose contains widgets used for composing a Matrix message.
package compose

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/mediautil"
	"github.com/pkg/errors"
)

// Composer is a message composer.
type Composer struct {
	*adw.Clamp
	box    *gtk.Box
	attach *gtk.Button
	input  *Input
}

// Controller describes the parent component that the Composer controls.
type Controller interface{}

const (
	ComposerMaxWidth   = 1000
	ComposerClampWidth = 800
)

var composerCSS = cssutil.Applier("composer", `
	.composer-attach {
		padding: 0px;
		min-width:  36px;
		min-height: 36px;
		margin:      0 12px;
		margin-right:  10px;
		border-radius: 99px;
	}
`)

// New creates a new Composer.
func New(ctx context.Context, ctrl Controller, roomID matrix.RoomID) *Composer {
	attach := gtk.NewButtonFromIconName("mail-attachment-symbolic")
	attach.SetSizeRequest(AvatarSize, -1)
	attach.SetVAlign(gtk.AlignCenter)
	attach.AddCSSClass("composer-attach")
	attach.Connect("clicked", func() { uploader(ctx, ctrl, roomID) })

	input := NewInput(ctx, roomID)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(attach)
	box.Append(input)
	composerCSS(box)

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(ComposerMaxWidth)
	clamp.SetTighteningThreshold(ComposerClampWidth)
	clamp.SetChild(box)

	return &Composer{
		Clamp:  clamp,
		box:    box,
		attach: attach,
		input:  input,
	}
}

func uploader(ctx context.Context, ctrl Controller, roomID matrix.RoomID) {
	chooser := gtk.NewFileChooserNative(
		"Upload File",
		app.Window(ctx),
		gtk.FileChooserActionOpen,
		"Upload", "Cancel",
	)
	chooser.SetSelectMultiple(false)

	// Cannot use chooser.File(); see
	// https://github.com/diamondburned/gotk4/issues/29.
	chooser.Connect("response", func(chooser *gtk.FileChooserNative, resp int) {
		if resp != int(gtk.ResponseAccept) {
			return
		}

		list := chooser.Files()
		length := list.NItems()
		if length == 0 {
			return
		}

		for i := uint(0); i < length; i++ {
			f := gio.File{Object: list.Item(i)}
			go upload(ctx, ctrl, roomID, f.Path())
		}
	})
	chooser.Show()
}

func upload(ctx context.Context, ctrl Controller, roomID matrix.RoomID, path string) {
	f, err := os.Open(path)
	if err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to open uploading file"))
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Ensure the file is closed when the context is cancelled. This will
	// interrupt the download.
	go func() {
		<-ctx.Done()
		f.Close()
	}()

	mime := mediautil.MIME(f)
	log.Println("mime type:", mime)
	client := gotktrix.FromContext(ctx)

	var uploader func(matrix.RoomID, gotrix.File) (matrix.EventID, error)

	switch strings.Split(mime, "/")[0] {
	case "image":
		uploader = client.SendImage
	case "video":
		uploader = client.SendVideo
	case "audio":
		uploader = client.SendAudio
	default:
		uploader = client.SendFile
	}

	_, err = uploader(roomID, gotrix.File{
		Name:     filepath.Base(path),
		Content:  f,
		MIMEType: mime,
	})
	if err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to upload file"))
	}
}
