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
	"github.com/diamondburned/gotk4/pkg/glib/v2"
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
	var fetched bool

	pic := gtk.NewPicture()
	pic.SetSizeRequest(100, 50)
	pic.SetCanShrink(true)
	pic.SetKeepAspectRatio(true)

	w := maxWidth * thumbnailScale
	h := maxHeight * thumbnailScale

	i, err := msg.ImageInfo()
	if err == nil && i.Width > 0 && i.Height > 0 {
		w, h = gotktrix.MaxSize(i.Width, i.Height, w, h)

		// Recalculate the max dimensions without scaling.
		_, actualHeight := gotktrix.MaxSize(i.Width, i.Height, maxWidth, maxHeight)
		pic.SetSizeRequest(100, actualHeight)

		if blur := renderBlurhash(msg.Info, w, h); blur != nil {
			pic.SetPaintable(blur)
		}
	}

	pic.Connect("map", func() {
		// Lazily fetch this image.
		if !fetched {
			fetched = true

			url, _ := gotktrix.FromContext(ctx).ImageThumbnail(msg, w, h)
			imgutil.AsyncGET(ctx, url, func(p gdk.Paintabler) {
				_, h := gotktrix.MaxSize(
					p.IntrinsicWidth(),
					p.IntrinsicHeight(),
					maxWidth,
					maxHeight,
				)
				pic.SetSizeRequest(100, h)
				pic.SetPaintable(p)
			})
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

const maxBlurhash = 50

func renderBlurhash(rawInfo json.RawMessage, w, h int) gdk.Paintabler {
	var info struct {
		BlurHash string `json:"xyz.amorgan.blurhash"`
	}

	if err := json.Unmarshal(rawInfo, &info); err != nil || info.BlurHash == "" {
		return nil
	}

	w, h = gotktrix.MaxSize(w, h, maxBlurhash, maxBlurhash)
	nrgba := image.NewNRGBA(image.Rect(0, 0, maxBlurhash, maxBlurhash))

	if err := blurhash.DecodeDraw(nrgba, info.BlurHash, 1); err != nil {
		return nil
	}

	return gdk.NewTextureForPixbuf(gdkpixbuf.NewPixbufFromBytes(
		glib.UseBytes(nrgba.Pix), gdkpixbuf.ColorspaceRGB, true, 8, w, h, nrgba.Stride,
	))
}
