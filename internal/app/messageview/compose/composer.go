// Package compose contains widgets used for composing a Matrix message.
package compose

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
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

	ctx context.Context
}

// Controller describes the parent component that the Composer controls.
type Controller interface {
	// AddEphemeralMessage(txID string, g gtk.Widgetter)
	ReplyTo(matrix.EventID)
	Edit(matrix.EventID)
}

const (
	ComposerMaxWidth   = 1000
	ComposerClampWidth = 800

	sendIcon  = "document-send-symbolic"
	editIcon  = "document-edit-symbolic"
	replyIcon = "mail-reply-sender-symbolic"
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
		ctx:    ctx,
	}

	gtkutil.BindActionMap(box, "composer", map[string]func(){
		"upload-file":   func() { uploader(ctx, ctrl, roomID) },
		"stop-replying": func() { ctrl.ReplyTo("") },
		"stop-editing":  func() { ctrl.Edit("") },
	})

	avatar.MenuItemsFunc(func() []gtkutil.PopoverMenuItem {
		items := make([]gtkutil.PopoverMenuItem, 0, 3)
		items = append(items,
			gtkutil.MenuItemIcon("Upload File", "composer.upload-file", "mail-attachment-symbolic"))

		if c.input.replyingTo != "" {
			items = append(items,
				gtkutil.MenuItem("Stop Replying", "composer.stop-replying"))
		}
		if c.input.editing != "" {
			items = append(items,
				gtkutil.MenuItem("Stop Editing", "composer.stop-editing"))
		}

		return items
	})

	return &c
}

// Input returns the composer's input.
func (c *Composer) Input() *Input {
	return c.input
}

// Edit switches the composer to edit mode and grabs an older message's body. If
// the message cannot be fetched from just the timeline state, then it will not
// be shown to the user. This means that editing backlog messages will behave
// weirdly.
//
// TODO(diamond): allow editing older messages.
// TODO(diamond): lossless Markdown editing (no mentions are lost).
func (c *Composer) Edit(eventID matrix.EventID) {
	c.input.editing = eventID
	c.input.replyingTo = ""

	if c.input.editing == "" {
		c.send.SetIconName(sendIcon)
		c.input.SetText("")
		return
	}

	c.send.SetIconName(editIcon)

	client := gotktrix.FromContext(c.ctx).Offline()
	revent := roomTimelineEvent(client, c.input.roomID, eventID)
	if revent == nil {
		return
	}

	msg, ok := revent.(event.RoomMessageEvent)
	if ok {
		c.input.SetText(msg.Body)
	}
}

func roomTimelineEvent(
	c *gotktrix.Client, roomID matrix.RoomID, eventID matrix.EventID) event.RoomEvent {

	events, _ := c.RoomTimeline(roomID)
	for _, ev := range events {
		if ev.ID() == eventID {
			return ev
		}
	}
	return nil
}

// ReplyTo sets the event ID that the to-be-sent message is supposed to be
// replying to. It replaces the previously-set event ID. The event ID is cleared
// when the message is sent. An empty string clears the replying state.
func (c *Composer) ReplyTo(eventID matrix.EventID) {
	c.input.editing = ""
	c.input.replyingTo = eventID

	if c.input.replyingTo == "" {
		c.send.SetIconName(sendIcon)
		return
	}

	c.send.SetIconName(replyIcon)
}
