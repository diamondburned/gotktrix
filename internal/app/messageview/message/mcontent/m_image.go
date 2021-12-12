package mcontent

import (
	"context"
	"encoding/json"
	"image"
	"log"

	"github.com/bbrks/go-blurhash"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

type imageContent struct {
	gtk.Widgetter
	ctx context.Context
	msg event.RoomMessageEvent

	picture *gtk.Picture
}

var imageCSS = cssutil.Applier("mcontent-image", `
	.mcontent-image {
		padding: 0;
		margin:  0;
		margin-top: 6px;
	}
`)

func newImageContent(ctx context.Context, msg event.RoomMessageEvent) contentPart {
	picture := gtk.NewPicture()
	picture.SetCanShrink(true)
	picture.SetCanFocus(false)
	picture.SetKeepAspectRatio(true)
	picture.SetHAlign(gtk.AlignStart)

	w := maxWidth
	h := maxHeight

	i, err := msg.ImageInfo()
	if err == nil && i.Width > 0 && i.Height > 0 {
		w, h = gotktrix.MaxSize(i.Width, i.Height, w, h)
		picture.SetSizeRequest(w, h)
		renderBlurhash(msg.Info, w, h, picture.SetPixbuf)
	}

	button := gtk.NewButton()
	button.AddCSSClass("mcontent-image")
	button.SetHAlign(gtk.AlignStart)
	button.SetHasFrame(false)
	button.SetChild(picture)
	button.SetTooltipText(msg.Body)
	button.Connect("clicked", func() {
		u, err := gotktrix.FromContext(ctx).MessageMediaURL(msg)
		if err != nil {
			log.Println("image URL error:", err)
			return
		}

		app.OpenURI(ctx, u)
	})

	return imageContent{
		Widgetter: button,
		ctx:       ctx,
		msg:       msg,
		picture:   picture,
	}
}

func (c imageContent) LoadMore() {
	client := gotktrix.FromContext(c.ctx)
	pw, ph := c.picture.SizeRequest()
	url, _ := client.ImageThumbnail(c.msg, pw, ph, gtkutil.ScaleFactor())
	imgutil.AsyncGET(c.ctx, url, c.picture.SetPaintable, imgutil.WithSizeOverrider(c.picture, pw, ph))
}

func (c imageContent) content() {}

const maxBlurhash = 25

func renderBlurhash(rawInfo json.RawMessage, w, h int, picFn func(*gdkpixbuf.Pixbuf)) {
	var info struct {
		BlurHash string `json:"xyz.amorgan.blurhash"`
	}

	if err := json.Unmarshal(rawInfo, &info); err != nil || info.BlurHash == "" {
		return
	}

	w, h = gotktrix.MaxSize(w, h, maxBlurhash, maxBlurhash)
	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))

	if err := blurhash.DecodeDraw(nrgba, info.BlurHash, 1); err != nil {
		return
	}

	picFn(gdkpixbuf.NewPixbufFromBytes(
		glib.NewBytesWithGo(nrgba.Pix), gdkpixbuf.ColorspaceRGB, true, 8, w, h, nrgba.Stride,
	))
}
