package compose

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/compose/autocomplete"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/md"
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

func copyMessage(buffer *gtk.TextBuffer, roomID matrix.RoomID) (event.RoomMessageEvent, bool) {
	head := buffer.StartIter()
	tail := buffer.EndIter()

	input := buffer.Text(&head, &tail, true)
	if input == "" {
		return event.RoomMessageEvent{}, false
	}

	ev := event.RoomMessageEvent{
		RoomEventInfo: event.RoomEventInfo{RoomID: roomID},
		Body:          input,
		MsgType:       event.RoomMessageText,
	}

	var html strings.Builder

	if err := md.Converter.Convert([]byte(input), &html); err == nil {
		var out string
		out = html.String()
		out = strings.TrimSpace(out)
		out = strings.TrimPrefix(out, "<p>") // we don't need these tags
		out = strings.TrimSuffix(out, "</p>")

		ev.Format = event.FormatHTML
		ev.FormattedBody = out
	}

	return ev, true
}

func customEmojiHTML(emoji autocomplete.EmojiData) string {
	if emoji.Unicode != "" {
		return emoji.Unicode
	}

	return fmt.Sprintf(
		`<img alt="%s" title="%[1]s" width="32" height="32" src="%s" data-mxc-emoticon/>`,
		html.EscapeString(string(emoji.Name)),
		html.EscapeString(string(emoji.Custom.URL)),
	)
}

const inlineEmojiSize = 18

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

	ac := autocomplete.New(tview, func(row autocomplete.SelectedData) bool {
		return finishAutocomplete(ctx, buffer, row)
	})
	ac.SetTimeout(time.Second)
	ac.Use(
		autocomplete.NewRoomMemberSearcher(ctx, roomID), // @
		autocomplete.NewEmojiSearcher(ctx, roomID),      // :
	)

	// Ugh. We have to be EXTREMELY careful with this context, because if it's
	// misused, it will put the input buffer into a very inconsistent state.
	// It must be invalidated every time to buffer changes, because we don't
	// want to risk

	buffer.Connect("changed", func(buffer *gtk.TextBuffer) {
		md.WYSIWYG(ctx, buffer)
		ac.Autocomplete(ctx)
	})

	send := gtk.NewButtonFromIconName("document-send-symbolic")
	send.SetTooltipText("Send")
	send.SetHasFrame(false)
	send.SetSizeRequest(AvatarWidth, -1)
	sendCSS(send)

	send.Connect("activate", func() {
		ev, ok := copyMessage(buffer, roomID)
		if !ok {
			return
		}

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
				if ac.IsVisible() {
					ac.Select()
					return true
				}

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
			case gdk.KEY_Escape:
				if ac.IsVisible() {
					ac.Clear()
					return true
				}
			case gdk.KEY_Up:
				return ac.MoveUp()
			case gdk.KEY_Down:
				return ac.MoveDown()
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

func finishAutocomplete(
	ctx context.Context, buffer *gtk.TextBuffer, row autocomplete.SelectedData) bool {

	switch data := row.Data.(type) {
	case autocomplete.RoomMemberData:
		log.Println("chose", data.ID)

	case autocomplete.EmojiData:
		// Delete the inserted text, which will equalize the two bounds. The
		// caller will use bounds[1], so we use that to revalidate it.
		buffer.Delete(row.Bounds[0], row.Bounds[1])
		if data.Unicode != "" {
			// Unicode emoji means we can just insert it in plain text.
			buffer.Insert(row.Bounds[1], data.Unicode, len(data.Unicode))
		} else {
			// Queue inserting the pixbuf.
			client := gotktrix.FromContext(ctx).Offline()
			url, _ := client.SquareThumbnail(data.Custom.URL, inlineEmojiSize)
			md.AsyncInsertImage(ctx, row.Bounds[1], url, imgutil.WithRectRescale(inlineEmojiSize))
			// Insert the HTML.
			md.InsertInvisible(row.Bounds[1], customEmojiHTML(data))
		}
	default:
		return false
	}

	return true
}

// requestAllMembers asynchronously fills up the local state with the given
// room's members.
func requestAllMembers(ctx context.Context, roomID matrix.RoomID) {
	client := gotktrix.FromContext(ctx)

	if err := client.RoomEnsureMembers(roomID); err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to prefetch members"))
	}
}
