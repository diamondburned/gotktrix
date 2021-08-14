package mcontent

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/mediautil"
)

type imageContent struct {
	gtk.Widgetter
}

var imageCSS = cssutil.Applier("mcontent-image", `
	.mcontent-image {
		border-radius: 0;
		padding: 0;
		margin:  0;
	}
`)

func newImageContent(ctx context.Context, msg event.RoomMessageEvent) contentPart {
	i, err := msg.ImageInfo()
	if err != nil {
		return newErroneousContent(err.Error(), -1, -1)
	}

	w, h := mediautil.MaxSize(i.Width, i.Height, maxWidth, maxHeight)

	var fetched bool

	img := gtk.NewImage()
	img.SetSizeRequest(w, h)
	img.Connect("map", func() {
		// Lazily fetch this image.
		if !fetched {
			url, _ := gotktrix.FromContext(ctx).Offline().Thumbnail(i.ThumbnailURL, w, h)
			imgutil.AsyncGET(ctx, url, img.SetFromPaintable)
			fetched = true
		}
	})

	button := gtk.NewButton()
	button.SetHAlign(gtk.AlignStart)
	button.SetHasFrame(false)
	button.SetChild(img)
	button.SetTooltipText(msg.Body)
	imageCSS(button)

	return imageContent{button}
}

func (c imageContent) content() {}
