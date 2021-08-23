package compose

import (
	"context"
	"io"
	"log"
	"mime"
	"strings"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/components/dialogs"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/diamondburned/gotktrix/internal/md"
	"github.com/diamondburned/gotktrix/internal/osutil"
	"github.com/pkg/errors"
)

// Input is the input component of the message composer.
type Input struct {
	*gtk.Box
	text *gtk.TextView
	send *gtk.Button

	buffer *gtk.TextBuffer

	ctx    context.Context
	roomID matrix.RoomID
}

var inputCSS = cssutil.Applier("composer-input", `
	.composer-input,
	.composer-input text {
		background-color: inherit;
	}
	.composer-input {
		padding: 12px 2px;
	}
`)

var sendCSS = cssutil.Applier("composer-send", `
	.composer-send {
		margin:   0px;
		padding: 10px;
		border-radius: 0;
	}
`)

func init() {
	md.TextTags.Combine(markuputil.TextTagsMap{
		// Not HTML tags.
		"_htmltag": {
			"family":     "Monospace",
			"foreground": "#808080",
		},
	})
}

func copyMessage(buffer *gtk.TextBuffer, roomID matrix.RoomID) event.RoomMessageEvent {
	head := buffer.StartIter()
	tail := buffer.EndIter()

	input := buffer.Text(&head, &tail, true)

	ev := event.RoomMessageEvent{
		RoomEventInfo: event.RoomEventInfo{RoomID: roomID},
		Body:          input,
		MsgType:       event.RoomMessageText,
	}

	var html strings.Builder

	if err := md.Converter.Convert([]byte(input), &html); err == nil {
		ev.Format = event.FormatHTML
		ev.FormattedBody = html.String()
	}

	return ev
}

// NewInput creates a new Input instance.
func NewInput(ctx context.Context, ctrl Controller, roomID matrix.RoomID) *Input {
	go requestAllMembers(ctx, roomID)

	tview := gtk.NewTextView()
	tview.SetWrapMode(gtk.WrapWordChar)
	tview.SetAcceptsTab(true)
	tview.SetHExpand(true)
	tview.SetInputHints(0 |
		gtk.InputHintEmoji |
		gtk.InputHintSpellcheck |
		gtk.InputHintWordCompletion |
		gtk.InputHintUppercaseSentences,
	)
	inputCSS(tview)

	buffer := tview.Buffer()
	buffer.Connect("changed", func(buffer *gtk.TextBuffer) {
		md.WYSIWYG(ctx, buffer)
		autocomplete(ctx, roomID, buffer)
	})

	send := gtk.NewButtonFromIconName("document-send-symbolic")
	send.SetTooltipText("Send")
	send.SetHasFrame(false)
	send.SetSizeRequest(AvatarWidth, -1)
	sendCSS(send)

	send.Connect("activate", func() {
		ev := copyMessage(buffer, roomID)

		head := buffer.StartIter()
		tail := buffer.EndIter()
		buffer.Delete(&head, &tail)

		go func() {
			client := gotktrix.FromContext(ctx)
			_, err := client.RoomEventSend(ev.RoomID, ev.Type(), ev)
			if err != nil {
				app.Error(ctx, errors.Wrap(err, "failed to send message"))
			}
		}()
	})

	enterKeyer := gtk.NewEventControllerKey()
	enterKeyer.Connect(
		"key-pressed",
		func(_ *gtk.EventControllerKey, val, code uint, state gdk.ModifierType) bool {
			switch val {
			case gdk.KEY_Return:
				// TODO: find a better way to do this. goldmark won't try to
				// parse an incomplete codeblock (I think), but the changed
				// signal will be fired after this signal.
				//
				// Perhaps we could use the FindChar method to avoid allocating
				// a new string (twice) on each keypress.
				head := buffer.StartIter()
				tail := buffer.IterAtOffset(buffer.ObjectProperty("cursor-position").(int))
				uinput := buffer.Text(&head, &tail, false)

				withinCodeblock := strings.Count(uinput, "```")%2 != 0

				// Enter (without holding Shift) sends the message.
				if !state.Has(gdk.ShiftMask) && !withinCodeblock {
					return send.Activate()
				}
			}

			return false
		},
	)

	tview.AddController(enterKeyer)

	tview.Connect("paste-clipboard", func() {
		display := gdk.DisplayGetDefault()

		clipboard := display.Clipboard()
		clipboard.ReadAsync(ctx, clipboard.Formats().MIMETypes(), 0, func(res gio.AsyncResulter) {
			mime, stream, err := clipboard.ReadFinish(res)
			if err != nil {
				app.Error(ctx, errors.Wrap(err, "failed to read clipboard"))
				return
			}

			if strings.Contains(mime, "text/plain") {
				// Ignore texts.
				stream.Close(ctx)
				return
			}

			promptUpload(ctx, roomID, uploadingFile{
				input:  stream,
				reader: gioutil.Reader(ctx, stream),
				mime:   mime,
				name:   "clipboard",
			})
		})
	})

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(tview)
	box.Append(send)

	return &Input{
		Box:    box,
		text:   tview,
		buffer: buffer,
		ctx:    ctx,
		roomID: roomID,
	}
}

// requestAllMembers asynchronously fills up the local state with the given
// room's members.
func requestAllMembers(ctx context.Context, roomID matrix.RoomID) {
	client := gotktrix.FromContext(ctx)

	if err := client.RoomEnsureMembers(roomID); err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to prefetch members"))
	}
}

type uploadingFile struct {
	input  gio.InputStreamer
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
	typ, _, err := mime.ParseMediaType(f.mime)
	if err != nil {
		app.Error(ctx, errors.Wrapf(err, "clipboard contains invalid MIME %q", f.mime))
		return
	}

	// Add a file extension if needed.
	if exts, _ := mime.ExtensionsByType(typ); len(exts) > 0 {
		f.name += exts[0]
	}

	bin := adw.NewBin()
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

func autocomplete(ctx context.Context, roomID matrix.RoomID, buf *gtk.TextBuffer) {

}
