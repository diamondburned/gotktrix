package compose

import (
	"context"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
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
func NewInput(ctx context.Context, roomID matrix.RoomID) *Input {
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

func autocomplete(ctx context.Context, roomID matrix.RoomID, buf *gtk.TextBuffer) {

}
