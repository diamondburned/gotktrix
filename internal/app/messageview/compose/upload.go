package compose

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"strings"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/event"
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
	"github.com/diamondburned/gotktrix/internal/components/progress"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/mediautil"
	"github.com/diamondburned/gotktrix/internal/locale"
	"github.com/diamondburned/gotktrix/internal/osutil"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

type uploadProgress struct {
	*progress.Bar
}

var uploadProgressCSS = cssutil.Applier("compose-upload-progress", `
	.compose-upload-progress {
		border-top: 1px solid @borders;
		padding:    5px 10px;
		padding-bottom: 10px;
	}
`)

func newUploadProgress(name string) *uploadProgress {
	bar := progress.NewBar()
	bar.SetText(name)
	bar.SetShowText(true)
	uploadProgressCSS(bar)

	return &uploadProgress{
		Bar: bar,
	}
}

// use sets the information from uploadingFile into the uploadProgress bar. A
// new uploadingFile is returned that wraps the file.
func (p *uploadProgress) use(r *uploadingFile) {
	p.SetText(r.name)

	if r.size > 0 {
		p.Bar.SetMax(r.size)
		p.Bar.SetLabelFunc(func(n, max int64) string {
			return fmt.Sprintf(
				"%s (%.0f%%, %s / %s)",
				r.name,
				float64(n)/float64(max)*100,
				humanize.Bytes(uint64(n)),
				humanize.Bytes(uint64(max)),
			)
		})
	}

	// Wrap the reader.
	reader := progress.WrapReader(r.ReadCloser, p.Bar)
	// Override the reader but keep the closer.
	r.ReadCloser = gioutil.ReadCloser(reader, r.ReadCloser)
}

// fileUpload describes a to-be-uploaded file.
type fileUpload struct {
	name string
	file func(context.Context) (*uploadingFile, error)
}

// uploadingFile describes an active file reader.
type uploadingFile struct {
	io.ReadCloser
	name string
	mime string
	size int64 // 0 if unknown, TODO
}

// newUploadingFile creates a new uploadingFile from the given gio.Filer, or nil
// if there's an error.
func newUploadingFile(ctx context.Context, file gio.Filer) (*uploadingFile, error) {
	s, err := file.Read(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read file")
	}

	return &uploadingFile{
		ReadCloser: gioutil.ReadCloser(
			gioutil.Reader(ctx, s),
			gioutil.InputCloser(ctx, s),
		),
		name: file.Basename(),
		mime: mediautil.FileMIME(ctx, s),
	}, nil
}

// newUploadingInput creates a new uploadingFile from the given InputStream and
// content type.
func newUploadingInput(ctx context.Context, input gio.InputStreamer, typ string) (*uploadingFile, error) {
	baseInput := gio.BaseInputStream(input)

	mimeType, _, err := mime.ParseMediaType(typ)
	if err != nil {
		baseInput.Close(ctx)
		return nil, errors.Wrapf(err, "clipboard contins invalid MIME type %q", typ)
	}

	// Add the file extension into the clipboard filename.
	name := "clipboard"
	if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
		name += exts[0]
	}

	return &uploadingFile{
		ReadCloser: gioutil.ReadCloser(
			gioutil.Reader(ctx, input),
			gioutil.InputCloser(ctx, input),
		),
		name: name,
		mime: mimeType,
	}, nil
}

type uploader struct {
	ctx    context.Context
	ctrl   Controller
	roomID matrix.RoomID
}

// ask creates a new file chooser asking the user to pick files to be uploaded.
func (u *uploader) ask() {
	// TODO: allow selecting multiple files
	chooser := gtk.NewFileChooserNative(
		"Upload File",
		app.Window(u.ctx),
		gtk.FileChooserActionOpen,
		locale.S(u.ctx, "Upload"), locale.S(u.ctx, "Cancel"),
	)
	chooser.SetSelectMultiple(false)

	// Cannot use chooser.File(); see
	// https://github.com/diamondburned/gotk4/issues/29.
	chooser.Connect("response", func(chooser *gtk.FileChooserNative, resp int) {
		if resp != int(gtk.ResponseAccept) {
			return
		}

		file := chooser.File()
		u.upload(fileUpload{
			name: file.Basename(),
			file: func(ctx context.Context) (*uploadingFile, error) {
				return newUploadingFile(ctx, file)
			},
		})
	})

	chooser.Show()
}

// paste pastes the content inside the clipboard. It ignores texts, since texts
// should be pasted into the composer instead.
func (u *uploader) paste() {
	display := gdk.DisplayGetDefault()

	clipboard := display.Clipboard()
	clipboard.ReadAsync(u.ctx, clipboard.Formats().MIMETypes(), 0, func(res gio.AsyncResulter) {
		typ, stream, err := clipboard.ReadFinish(res)
		if err != nil {
			app.Error(u.ctx, errors.Wrap(err, "failed to read clipboard"))
			return
		}

		log.Println("clipboard type =", typ)

		baseStream := gio.BaseInputStream(stream)

		// How is utf8_string a valid MIME type? GTK, what the fuck?
		if strings.HasPrefix(typ, "text") || typ == "utf8_string" {
			baseStream.Close(u.ctx)
			return
		}

		u.promptUpload(fileUpload{
			name: "clipboard",
			file: func(ctx context.Context) (*uploadingFile, error) {
				return newUploadingInput(ctx, stream, typ)
			},
		})
	})
}

func (u *uploader) promptUpload(file fileUpload) {
	bin := adaptive.NewBin()
	bin.SetHAlign(gtk.AlignCenter)
	bin.SetVAlign(gtk.AlignCenter)

	content := gtk.NewScrolledWindow()
	content.SetHExpand(true)
	content.SetVExpand(true)
	content.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	content.SetChild(bin)

	d := dialogs.New(app.Window(u.ctx), "Cancel", "Upload")
	d.SetDefaultSize(300, 200)
	d.SetTitle("Upload File")
	d.SetChild(content)
	d.BindEnterOK()
	d.BindCancelClose()

	// Disable uploading until the reader is fully consumed, because we
	// can't upload a partially read stream.
	d.OK.SetSensitive(false)

	loading := gtk.NewSpinner()
	loading.SetSizeRequest(24, 24)
	loading.Start()
	bin.SetChild(loading)

	useStatusPage := func(icon string) {
		img := gtk.NewImageFromIconName(icon)
		img.SetIconSize(gtk.IconSizeLarge)

		label := gtk.NewLabel(file.name)
		label.SetXAlign(0)
		label.SetEllipsize(pango.EllipsizeEnd)
		label.SetAttributes(markuputil.Attrs(
			pango.NewAttrScale(1.1),
		))

		box := gtk.NewBox(gtk.OrientationHorizontal, 2)
		box.Append(img)
		box.Append(label)
		bin.SetChild(box)

		loading.Stop()
		d.OK.SetSensitive(true)
	}

	var upload *uploadingFile
	ctx, cancel := context.WithCancel(u.ctx)

	close := func() {
		// Ensure the fd is closed if any.
		if upload != nil {
			upload.Close()
		}

		cancel()
		d.Close()
	}

	d.Cancel.ConnectClicked(close)

	gtkutil.Async(context.Background(), func() func() {
		var err error

		upload, err = file.file(ctx)
		if err != nil {
			// system error, probably
			app.Error(u.ctx, errors.Wrap(err, "cannot prompt for file upload"))
			return close
		}

		// See if we can make an image thumbnail.
		switch strings.Split(upload.mime, "/")[0] {
		case "image":
			r, err := osutil.Consume(upload)

			upload.Close()
			upload.ReadCloser = r

			if err != nil {
				// This is an error worth notifying the user for, because the
				// data to be uploaded will definitely be corrupted.
				app.Error(u.ctx, errors.Wrap(err, "corrupted data reading clipboard"))
				// Close the dialog.
				return close
			}

			p, err := imgutil.Read(r)
			r.Rewind()

			if err != nil {
				log.Println("image thumbnailing error:", err)
				return func() { useStatusPage("image-x-generic") }
			}

			return func() {
				img := gtk.NewPicture()
				img.SetKeepAspectRatio(true)
				img.SetCanShrink(true)
				img.SetTooltipText(upload.name)
				img.SetHExpand(true)
				img.SetVExpand(true)
				img.SetPaintable(p)
				bin.SetChild(img)

				loading.Stop()
				d.OK.SetSensitive(true)
			}
		default:
			return func() { useStatusPage("x-office-document") }
		}
	})

	d.OK.ConnectClicked(func() {
		u.uploadKnown(upload)
		d.Close()
	})
	d.Show()
}

func (u *uploader) upload(file fileUpload) {
	bar := newUploadProgress(file.name)

	ev := newRoomMessageEvent(gotktrix.FromContext(u.ctx), u.roomID)
	ev.MessageType = event.RoomMessageFile // whatever

	mark := u.ctrl.AddSendingMessageCustom(&ev, bar)

	go func() {
		upload, err := file.file(u.ctx)

		glib.IdleAdd(func() {
			if err != nil {
				bar.Error(err)
				return
			}

			bar.use(upload)
			u.finishUpload(mark, upload, bar)
		})
	}()
}

func (u *uploader) uploadKnown(upload *uploadingFile) {
	bar := newUploadProgress(upload.name)
	bar.use(upload)

	ev := newRoomMessageEvent(gotktrix.FromContext(u.ctx), u.roomID)
	ev.MessageType = event.RoomMessageFile // whatever

	mark := u.ctrl.AddSendingMessageCustom(&ev, bar)
	u.finishUpload(mark, upload, bar)
}

func (u *uploader) finishUpload(mark interface{}, upload *uploadingFile, bar *uploadProgress) {
	go func() {
		client := gotktrix.FromContext(u.ctx)
		file := gotrix.File{
			Name:     upload.name,
			MIMEType: upload.mime,
			Content:  upload.ReadCloser,
		}

		var eventID matrix.EventID
		var err error

		switch strings.Split(upload.mime, "/")[0] {
		case "image":
			eventID, err = client.SendImage(u.roomID, file)
		case "audio":
			eventID, err = client.SendAudio(u.roomID, file)
		case "video":
			eventID, err = client.SendVideo(u.roomID, file)
		default:
			eventID, err = client.SendFile(u.roomID, file)
		}

		glib.IdleAdd(func() {
			if err != nil {
				bar.Error(err)
			} else {
				u.ctrl.BindSendingMessage(mark, eventID)
			}
		})
	}()
}
