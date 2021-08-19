package mcontent

import (
	"context"
	"encoding/json"
	"image"
	"log"

	"github.com/bbrks/go-blurhash"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

type imageContent struct {
	gtk.Widgetter
}

var imageCSS = cssutil.Applier("mcontent-image", `
	.mcontent-image {
		background-color: @theme_bg_color;
		border: 1px solid @theme_base_color;
	}
	.mcontent-image button {
		border-radius: 0;
		padding: 0;
		margin:  0;
	}
`)

const thumbnailScale = 2

func newImageContent(ctx context.Context, msg event.RoomMessageEvent) contentPart {
	i, err := msg.ImageInfo()
	if err != nil {
		return newErroneousContent(err.Error(), -1, -1)
	}

	w, h := gotktrix.MaxSize(i.Width, i.Height, maxWidth, maxHeight)

	var fetched bool

	pic := gtk.NewPicture()
	pic.SetSizeRequest(-1, h) // allow flexible width
	pic.SetCanShrink(true)
	pic.SetKeepAspectRatio(true)

	if w > 0 && h > 0 {
		w *= thumbnailScale
		h *= thumbnailScale

		if blur := renderBlurhash(msg.Info, w, h); blur != nil {
			pic.SetPaintable(blur)
		}
	}

	pic.Connect("map", func() {
		// Lazily fetch this image.
		if !fetched {
			fetched = true

			url, _ := gotktrix.FromContext(ctx).ImageThumbnail(msg, w, h)
			imgutil.AsyncGET(ctx, url, pic.SetPaintable)
		}
	})

	button := gtk.NewButton()
	button.SetHasFrame(false)
	button.SetChild(pic)
	button.SetTooltipText(msg.Body)
	button.Connect("clicked", func() {
		u, err := gotktrix.FromContext(ctx).MessageMediaURL(msg)
		if err != nil {
			log.Println("image URL error:", err)
			return
		}

		app.OpenURI(ctx, u)
	})

	bin := adw.NewBin()
	bin.SetHAlign(gtk.AlignStart)
	bin.SetChild(button)
	imageCSS(bin)

	return imageContent{bin}
}

func (c imageContent) content() {}

func renderBlurhash(rawInfo json.RawMessage, w, h int) gdk.Paintabler {
	var info struct {
		BlurHash string `json:"xyz.amorgan.blurhash"`
	}

	if err := json.Unmarshal(rawInfo, &info); err != nil || info.BlurHash == "" {
		return nil
	}

	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))

	if err := blurhash.DecodeDraw(nrgba, info.BlurHash, 1); err != nil {
		return nil
	}

	return gdk.NewTextureForPixbuf(gdkpixbuf.NewPixbufFromBytes(
		nrgba.Pix, gdkpixbuf.ColorspaceRGB, true, 8, w, h, nrgba.Stride,
	))
}
