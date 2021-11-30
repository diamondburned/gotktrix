package compose

import (
	"context"
	"io"
	"log"
	"mime"
	"strings"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/components/dialogs"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/mediautil"
	"github.com/diamondburned/gotktrix/internal/osutil"
	"github.com/pkg/errors"
)

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

		go upload(ctx, ctrl, roomID, chooser.File())
	})
	chooser.Show()
}

func upload(ctx context.Context, ctrl Controller, roomID matrix.RoomID, f gio.Filer) {
	s, err := f.Read(ctx)
	if err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to open file stream"))
		return
	}
	defer s.Close(ctx)

	mime := mediautil.FileMIME(ctx, s)
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
		Name:     f.Basename(),
		Content:  gioutil.Reader(ctx, s),
		MIMEType: mime,
	})
	if err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to upload file"))
	}
}

type uploadingFile struct {
	input  *gio.InputStream
	reader io.ReadCloser
	name   string
	mime   string
}

func (f *uploadingFile) Close() error {
	f.input.Close(context.Background())
	f.reader.Close()
	return nil
}

func promptUpload(ctx context.Context, room matrix.RoomID, f uploadingFile) {
	// Add a file extension if needed.
	if exts, _ := mime.ExtensionsByType(f.mime); len(exts) > 0 {
		f.name += exts[0]
	}

	bin := adaptive.NewBin()
	bin.SetHAlign(gtk.AlignCenter)
	bin.SetVAlign(gtk.AlignCenter)

	content := gtk.NewScrolledWindow()
	content.SetHExpand(true)
	content.SetVExpand(true)
	content.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	content.SetChild(bin)

	d := dialogs.New(app.Window(ctx), "Cancel", "Upload")
	d.SetDefaultSize(300, 200)
	d.SetTitle("Upload File")
	d.SetChild(content)
	d.BindEnterOK()

	useStatusPage := func(icon string) {
		img := gtk.NewImageFromIconName(icon)
		img.SetIconSize(gtk.IconSizeLarge)

		label := gtk.NewLabel(f.name)
		label.SetXAlign(0)
		label.SetEllipsize(pango.EllipsizeEnd)
		label.SetAttributes(markuputil.Attrs(
			pango.NewAttrScale(1.1),
		))

		box := gtk.NewBox(gtk.OrientationHorizontal, 2)
		box.Append(img)
		box.Append(label)
		bin.SetChild(box)
	}

	// See if we can make an image thumbnail.
	switch strings.Split(f.mime, "/")[0] {
	case "image":
		// Disable uploading until the reader is fully consumed, because we
		// can't upload a partially read stream.
		d.OK.SetSensitive(false)

		loading := gtk.NewSpinner()
		loading.SetSizeRequest(24, 24)
		loading.Start()
		bin.SetChild(loading)

		fallback := func() {
			useStatusPage("image-x-generic")

			loading.Stop()
			d.OK.SetSensitive(true)
		}

		// done is called in a goroutine.
		done := func(r io.ReadCloser, p gdk.Paintabler, err error) {
			if err != nil {
				log.Println("image thumbnailing error:", err)
				glib.IdleAdd(func() {
					f.reader = r
					fallback()
				})
				return
			}

			glib.IdleAdd(func() {
				f.reader = r

				img := gtk.NewPicture()
				img.SetKeepAspectRatio(true)
				img.SetCanShrink(true)
				img.SetTooltipText(f.name)
				img.SetHExpand(true)
				img.SetVExpand(true)
				img.SetPaintable(p)
				bin.SetChild(img)

				loading.Stop()
				d.OK.SetSensitive(true)
			})
		}

		go func() {
			t, err := osutil.Consume(f.reader)
			if err != nil {
				// This is an error worth notifying the user for, because the
				// data to be uploaded will definitely be corrupted.
				app.Error(ctx, errors.Wrap(err, "corrupted data reading clipboard"))
				// Activate the cancel button to clean up the readers and close
				// the dialog.
				glib.IdleAdd(func() { d.Cancel.Activate() })
				return
			}

			r, err := t.Open()
			if err != nil {
				done(t, nil, err)
				return
			}

			p, err := imgutil.Read(r)
			r.Close()

			done(t, p, err)
		}()

	default:
		useStatusPage("x-office-document")
	}

	d.Cancel.Connect("clicked", func() {
		f.Close()
		d.Close()
	})

	d.OK.Connect("clicked", func() {
		go func() {
			startUpload(ctx, room, f)
			f.Close()
		}()
		d.Close()
	})

	d.Show()
}

func startUpload(ctx context.Context, room matrix.RoomID, f uploadingFile) {
	client := gotktrix.FromContext(ctx)
	file := gotrix.File{
		Name:     f.name,
		MIMEType: f.mime,
		Content:  f.reader,
	}

	var err error

	switch strings.Split(f.mime, "/")[0] {
	case "image":
		_, err = client.SendImage(room, file)
	case "audio":
		_, err = client.SendAudio(room, file)
	case "video":
		_, err = client.SendVideo(room, file)
	default:
		_, err = client.SendFile(room, file)
	}

	if err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to upload clipboard"))
	}
}
