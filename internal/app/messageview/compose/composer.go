// Package compose contains widgets used for composing a Matrix message.
package compose

import (
	"context"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// Composer is a message composer.
type Composer struct {
	*adw.Clamp
	box    *gtk.Box
	avatar *Avatar
	input  *Input
	send   *gtk.Button

	replyingTo matrix.EventID
}

// Controller describes the parent component that the Composer controls.
type Controller interface {
	// AddEphemeralMessage(txID string, g gtk.Widgetter)
	ReplyTo(matrix.EventID)
}

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
	avatar := NewAvatar(ctx, roomID)

	input := NewInput(ctx, ctrl, roomID)

	rbox := gtk.NewBox(gtk.OrientationVertical, 0)
	rbox.Append(input)
	rbox.Append(gtk.NewLabel("")) // TODO: typing signals

	send := gtk.NewButtonFromIconName(sendIcon)
	send.SetTooltipText("Send")
	send.SetHasFrame(false)
	send.SetSizeRequest(AvatarWidth, -1)
	send.Connect("clicked", func() { input.Send() })
	sendCSS(send)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(avatar)
	box.Append(rbox)
	box.Append(send)
	composerCSS(box)

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(ComposerMaxWidth)
	clamp.SetTighteningThreshold(ComposerClampWidth)
	clamp.SetChild(box)

	c := Composer{
		Clamp:  clamp,
		box:    box,
		avatar: avatar,
		input:  input,
		send:   send,
	}

	gtkutil.BindActionMap(box, "composer", map[string]func(){
		"upload-file":   func() { uploader(ctx, ctrl, roomID) },
		"stop-replying": func() { ctrl.ReplyTo("") },
	})

	avatar.MenuItemsFunc(func() []gtkutil.PopoverMenuItem {
		items := make([]gtkutil.PopoverMenuItem, 0, 2)
		items = append(items,
			gtkutil.MenuItemIcon("Upload File", "composer.upload-file", "mail-attachment-symbolic"))

		if c.replyingTo != "" {
			items = append(items,
				gtkutil.MenuItem("Stop Replying", "composer.stop-replying"))
		}

		return items
	})

	return &c
}

// Input returns the composer's input.
func (c *Composer) Input() *Input {
	return c.input
}

// ReplyTo sets the event ID that the to-be-sent message is supposed to be
// replying to. It replaces the previously-set event ID. The event ID is cleared
// when the message is sent. An empty string clears the replying state.
func (c *Composer) ReplyTo(eventID matrix.EventID) {
	c.replyingTo = eventID

	if c.replyingTo != "" {
		c.send.SetIconName(replyIcon)
	} else {
		c.send.SetIconName(sendIcon)
	}
}
