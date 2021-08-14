package compose

import (
	"context"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
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

// NewInput creates a new Input instance.
func NewInput(ctx context.Context, roomID matrix.RoomID) *Input {
	text := gtk.NewTextView()
	text.SetHExpand(true)
	inputCSS(text)

	send := gtk.NewButtonFromIconName("document-send-symbolic")
	send.SetTooltipText("Send")
	send.SetHasFrame(false)
	send.SetSizeRequest(AvatarWidth, -1)
	sendCSS(send)

	buffer := text.Buffer()

	enterKeyer := gtk.NewEventControllerKey()
	enterKeyer.Connect(
		"key-pressed",
		func(_ *gtk.EventControllerKey, val, code uint, state gdk.ModifierType) bool {
			// Enter (without holding Shift) sends the message.
			if val == gdk.KEY_Return && !state.Has(gdk.ShiftMask) {
				return send.Activate()
			}

			return false
		},
	)

	text.AddController(enterKeyer)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(text)
	box.Append(send)

	return &Input{
		Box:    box,
		text:   text,
		buffer: buffer,
		ctx:    ctx,
		roomID: roomID,
	}
}
