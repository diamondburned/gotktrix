package compose

import (
	"context"
	"fmt"
	"io"
	"mime"
	"strings"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/dialogs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/mediautil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotkit/utils/osutil"
	"github.com/diamondburned/gotktrix/internal/components/filepick"
	"github.com/diamondburned/gotktrix/internal/components/progress"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

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

	var size int64

	info, err := file.QueryInfo(ctx, gio.FILE_ATTRIBUTE_STANDARD_SIZE, 0)
	if err == nil {
		size = info.Size()
	}

	return &uploadingFile{
		ReadCloser: gioutil.ReadCloser(
			gioutil.Reader(ctx, s),
			gioutil.InputCloser(ctx, s),
		),
		name: file.Basename(),
		mime: mediautil.FileMIME(ctx, file),
		size: size,
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

	var size int64

	// Query the size if the input is a file input. This actually never hits,
	// but maybe one day we'll make it work.
	if file, ok := input.(*gio.FileInputStream); ok {
		info, err := file.QueryInfo(ctx, gio.FILE_ATTRIBUTE_STANDARD_SIZE)
		if err == nil {
			size = info.Size()
		}
	}

	return &uploadingFile{
		ReadCloser: gioutil.ReadCloser(
			gioutil.Reader(ctx, input),
			gioutil.InputCloser(ctx, input),
		),
		name: name,
		mime: mimeType,
		size: size,
	}, nil
}

type uploadProgress struct {
	*progress.Bar
}

var uploadProgressCSS = cssutil.Applier("compose-upload-progress", `
	.compose-upload-progress {
		border-top:    1px solid @borders;
		border-bottom: 1px solid transparent;
		padding:    5px 10px;
		padding-bottom: 10px;
	}
	.messageview-messagerow:not(:last-child) .compose-upload-progress {
		border-bottom: 1px solid @borders;
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
	} else {
		p.SetText(r.name)
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

type uploader struct {
	ctx    context.Context
	ctrl   Controller
	roomID matrix.RoomID
}

// ask creates a new file chooser asking the user to pick files to be uploaded.
func (u uploader) ask() {
	// TODO: allow selecting multiple files
	chooser := filepick.New(
		u.ctx, "Upload File",
		gtk.FileChooserActionOpen,
		locale.S(u.ctx, "Upload"),
		locale.S(u.ctx, "Cancel"),
	)
	chooser.SetSelectMultiple(false)

	// Cannot use chooser.File(); see
	// https://github.com/diamondburned/gotk4/issues/29.
	chooser.ConnectAccept(func() {
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
func (u uploader) paste() {
	display := gdk.DisplayGetDefault()

	clipboard := display.Clipboard()
	mimeTypes := clipboard.Formats().MIMETypes()

	// Ignore anything text.
	for _, mime := range mimeTypes {
		if mimeIsText(mime) {
			return
		}
	}

	clipboard.ReadAsync(u.ctx, mimeTypes, 0, func(res gio.AsyncResulter) {
		typ, stream, err := clipboard.ReadFinish(res)
		if err != nil {
			app.Error(u.ctx, errors.Wrap(err, "failed to read clipboard"))
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

func mimeIsText(mime string) bool {
	// How is utf8_string a valid MIME type? GTK, what the fuck?
	return strings.HasPrefix(mime, "text") || mime == "utf8_string"
}

func (u uploader) promptUpload(file fileUpload) {
	bin := adaptive.NewBin()
	bin.SetHAlign(gtk.AlignCenter)
	bin.SetVAlign(gtk.AlignCenter)

	content := gtk.NewScrolledWindow()
	content.SetHExpand(true)
	content.SetVExpand(true)
	content.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	content.SetChild(bin)

	d := dialogs.New(u.ctx, locale.S(u.ctx, "Cancel"), locale.S(u.ctx, "Upload"))
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
		label.SetAttributes(textutil.Attrs(
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

			return func() {
				img := gtk.NewPicture()
				img.SetKeepAspectRatio(true)
				img.SetCanShrink(true)
				img.SetTooltipText(upload.name)
				img.SetHExpand(true)
				img.SetVExpand(true)
				img.SetFilename(r.Name())
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

func (u uploader) upload(file fileUpload) {
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

func (u uploader) uploadKnown(upload *uploadingFile) {
	bar := newUploadProgress(upload.name)
	bar.use(upload)

	ev := newRoomMessageEvent(gotktrix.FromContext(u.ctx), u.roomID)
	ev.MessageType = event.RoomMessageFile // whatever

	mark := u.ctrl.AddSendingMessageCustom(&ev, bar)
	u.finishUpload(mark, upload, bar)
}

func (u uploader) finishUpload(mark interface{}, upload *uploadingFile, bar *uploadProgress) {
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
