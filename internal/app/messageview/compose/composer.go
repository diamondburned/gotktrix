// Package compose contains widgets used for composing a Matrix message.
package compose

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
)

// Composer is a message composer.
type Composer struct {
	*gtk.Box
	iscroll *gtk.ScrolledWindow
	input   *Input
	send    *gtk.Button

	ctx    context.Context
	ctrl   Controller
	roomID matrix.RoomID

	action struct {
		*gtk.Button
		upload  ActionData
		current func()
	}
	editing bool
}

// Controller describes the parent component that the Composer controls.
type Controller interface {
	message.MessageViewer
	// FocusLatestUserEventID returns the latest event ID of the current user,
	// or an empty string if none.
	FocusLatestUserEventID() matrix.EventID
	// AddSendingMessage adds the given RawEvent as a sending message and
	// returns a mark that is given to BindSendingMessage.
	AddSendingMessage(ev event.RoomEvent) (mark interface{})
	// AddSendingMessageCustom adds the given RawEvent as a sending message and
	// the given widget as the widget body, returning a mark that is given to
	// BindSendingMessage.
	AddSendingMessageCustom(ev event.RoomEvent, w gtk.Widgetter) (mark interface{})
	// StopSendingMessage stops sending the message with the given mark.
	StopSendingMessage(mark interface{}) bool
	// BindSendingMessage takes in the mark value returned by AddSendingMessage.
	BindSendingMessage(mark interface{}, evID matrix.EventID) (replaced bool)
}

// inputController wraps a Composer and Controller to implement InputController.
type inputController struct {
	Controller
	composer *Composer
}

func (c *inputController) IsEditing() bool {
	return c.composer.editing
}

const (
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
	.composer-action {
		padding: 4px;
		margin-top:   7px; /* why 7 */
		margin-left:  14px;
		margin-right: 8px;
	}
	.composer-input-placeholder {
		padding: 0px 2px; /* Keep the same as .composer-input */
		padding-top: 12px;
		color: alpha(@theme_fg_color, 0.65);
	}
`)

var sendCSS = cssutil.Applier("composer-send", `
	.composer-send {
		margin:   0px;
		padding: 10px;
		border-radius: 0;
		min-height: 0;
		min-width:  0;
	}
`)

// New creates a new Composer.
func New(ctx context.Context, ctrl Controller, roomID matrix.RoomID) *Composer {
	c := Composer{
		ctx:    ctx,
		ctrl:   ctrl,
		roomID: roomID,
	}

	// TODO: turn this into a single action button. There's no point in having a
	// menu that has only 1 working item.
	c.action.Button = gtk.NewButton()
	c.action.SetVAlign(gtk.AlignStart)
	c.action.SetHasFrame(false)
	c.action.AddCSSClass("composer-action")

	c.input = NewInput(ctx, &inputController{ctrl, &c}, roomID)
	c.input.SetVScrollPolicy(gtk.ScrollNatural)

	roomName, _ := gotktrix.FromContext(ctx).Offline().RoomName(roomID)

	placeholder := gtk.NewLabel(locale.Sprintf(ctx, "Message %s", roomName))
	placeholder.AddCSSClass("composer-input-placeholder")
	placeholder.SetVAlign(gtk.AlignStart)
	placeholder.SetHAlign(gtk.AlignStart)
	placeholder.SetEllipsize(pango.EllipsizeEnd)

	revealer := gtk.NewRevealer()
	revealer.SetChild(placeholder)
	revealer.SetCanTarget(false)
	revealer.SetRevealChild(true)
	revealer.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	revealer.SetTransitionDuration(50)

	overlay := gtk.NewOverlay()
	overlay.SetChild(c.input)
	overlay.AddOverlay(revealer)
	overlay.SetClipOverlay(revealer, true)

	// Show or hide the placeholder when the buffer is empty or not.
	c.input.buffer.ConnectChanged(func() {
		start, end := c.input.buffer.Bounds()
		// Reveal if the buffer has 0 length.
		revealer.SetRevealChild(start.Offset() == end.Offset())
	})

	c.iscroll = gtk.NewScrolledWindow()
	c.iscroll.AddCSSClass("composer-input-scroll")
	c.iscroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	c.iscroll.SetPropagateNaturalHeight(true)
	c.iscroll.SetMaxContentHeight(500)
	c.iscroll.SetChild(overlay)

	c.send = gtk.NewButtonFromIconName(sendIcon)
	c.send.SetTooltipText("Send")
	c.send.SetHasFrame(false)
	c.send.Connect("clicked", func() { c.input.Send() })
	sendCSS(c.send)

	c.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	c.Append(c.action)
	c.Append(c.iscroll)
	c.Append(c.send)
	c.SetFocusChild(c.iscroll)
	composerCSS(c.Box)

	// gtkutil.BindActionMap(box, "composer", map[string]func(){
	// 	"upload-file":   func() { c.upload.ask() },
	// 	"stop-replying": func() { ctrl.ReplyTo("") },
	// 	"stop-editing":  func() { ctrl.Edit("") },
	// })

	c.action.upload = ActionData{
		Name: "Upload File",
		Icon: "list-add-symbolic",
		Func: func() { c.uploader().ask() },
	}

	c.action.Connect("clicked", func() { c.action.current() })
	c.setAction(c.action.upload)

	return &c
}

func (c *Composer) uploader() uploader {
	return uploader{
		ctx:    c.ctx,
		ctrl:   c.ctrl,
		roomID: c.roomID,
	}
}

// Input returns the composer's input.
func (c *Composer) Input() *Input {
	return c.input
}

// ActionData is the data that the action button in the composer bar is
// currently doing.
type ActionData struct {
	Icon string
	Name string
	Func func()
}

// setAction sets the action of the button in the composer.
func (c *Composer) setAction(action ActionData) {
	c.action.SetSensitive(action.Func != nil)
	c.action.SetIconName(action.Icon)
	c.action.SetTooltipText(action.Name)
	c.action.current = action.Func
}

// Edit switches the composer to edit mode and grabs an older message's body. If
// the message cannot be fetched from just the timeline state, then it will not
// be shown to the user. This means that editing backlog messages will behave
// weirdly.
//
// TODO(diamond): allow editing older messages.
// TODO(diamond): lossless Markdown editing (no mentions are lost).
func (c *Composer) Edit(eventID matrix.EventID) bool {
	c.editing = c.edit(eventID)
	return c.editing
}

func (c *Composer) edit(eventID matrix.EventID) bool {
	c.input.editing = eventID
	c.input.replyingTo = ""

	if c.input.editing == "" {
		c.send.SetIconName(sendIcon)
		c.input.SetText("")
		c.setAction(c.action.upload)
		return false
	}

	client := gotktrix.FromContext(c.ctx).Offline()
	revent := roomTimelineEvent(client, c.input.roomID, eventID)
	if revent == nil {
		c.input.editing = ""
		return false
	}

	msg, ok := revent.(*event.RoomMessageEvent)
	if !ok {
		c.input.editing = ""
		return false
	}

	c.setAction(ActionData{
		Name: "Stop Editing",
		Icon: "edit-clear-all-symbolic",
		Func: func() { c.ctrl.Edit("") },
	})

	c.send.SetIconName(editIcon)
	c.input.SetText(msg.Body)

	return true
}

func roomTimelineEvent(
	c *gotktrix.Client, roomID matrix.RoomID, eventID matrix.EventID) event.RoomEvent {

	events, _ := c.RoomTimeline(roomID)
	for _, ev := range events {
		if ev.RoomInfo().ID == eventID {
			return ev
		}
	}
	return nil
}

// ReplyTo sets the event ID that the to-be-sent message is supposed to be
// replying to. It replaces the previously-set event ID. The event ID is cleared
// when the message is sent. An empty string clears the replying state.
func (c *Composer) ReplyTo(eventID matrix.EventID) bool {
	c.input.editing = ""
	c.input.replyingTo = eventID

	if c.input.replyingTo == "" {
		c.send.SetIconName(sendIcon)
		return false
	}

	c.setAction(ActionData{
		Name: "Stop Replying",
		Icon: "edit-clear-all-symbolic",
		Func: func() { c.ctrl.ReplyTo("") },
	})

	c.send.SetIconName(replyIcon)
	return true
}
