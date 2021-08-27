// Package compose contains widgets used for composing a Matrix message.
package compose

import (
	"context"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// Composer is a message composer.
type Composer struct {
	*adw.Clamp
	box    *gtk.Box
	attach *gtk.Button
	input  *Input
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
	attach := gtk.NewButtonFromIconName("mail-attachment-symbolic")
	attach.SetSizeRequest(AvatarSize, -1)
	attach.SetVAlign(gtk.AlignCenter)
	attach.AddCSSClass("composer-attach")
	attach.Connect("clicked", func() { uploader(ctx, ctrl, roomID) })

	input := NewInput(ctx, ctrl, roomID)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(attach)
	box.Append(input)
	composerCSS(box)

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(ComposerMaxWidth)
	clamp.SetTighteningThreshold(ComposerClampWidth)
	clamp.SetChild(box)

	return &Composer{
		Clamp:  clamp,
		box:    box,
		attach: attach,
		input:  input,
	}
}

// Input returns the composer's input.
func (c *Composer) Input() *Input {
	return c.input
}
