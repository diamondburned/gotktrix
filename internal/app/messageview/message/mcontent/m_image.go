package mcontent

import (
	"context"
	"encoding/json"
	"image"
	"log"

	"github.com/bbrks/go-blurhash"
	"github.com/diamondburned/chatkit/components/embed"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
)

type imageContent struct {
	*embed.Embed
	ctx context.Context
	msg *event.RoomMessageEvent
}

var imageCSS = cssutil.Applier("mcontent-image", `
	.mcontent-image {
		margin-top: 6px;
	}
`)

func newImageContent(ctx context.Context, msg *event.RoomMessageEvent) *imageContent {
	embed := embed.New(ctx, maxWidth, maxHeight, embed.Opts{
		Type:     embed.EmbedTypeImage,
		Provider: imgutil.HTTPProvider,
		CanHide:  true,
	})
	embed.AddCSSClass("mcontent-image-content")
	embed.SetOpenURL(func() {
		u, err := gotktrix.FromContext(ctx).MessageMediaURL(msg)
		if err != nil {
			log.Println("image URL error:", err)
			return
		}
		app.OpenURI(ctx, u)
	})

	c := imageContent{
		Embed: embed,
		ctx:   ctx,
		msg:   msg,
	}

	i, err := msg.ImageInfo()
	if err == nil && i.Width > 0 && i.Height > 0 {
		embed.SetSizeRequest(i.Width, i.Height)
	} else {
		// Oversize and resize it back after.
		embed.SetSizeRequest(i.Width, i.Height)
	}

	return &c
}

func (c *imageContent) LoadMore() {
	if w, h := c.Embed.Size(); w > 0 && h > 0 {
		renderBlurhash(c.msg.AdditionalInfo, w, h, func(p *gdkpixbuf.Pixbuf) {
			c.Embed.SetFromURL("")
			c.Embed.Thumbnail.SetPixbuf(p)
		})
	}

	client := gotktrix.FromContext(c.ctx)
	url, _ := client.ImageThumbnail(c.msg, maxWidth, maxHeight, gtkutil.ScaleFactor())
	c.Embed.SetFromURL(url)
}

func (c *imageContent) content() {}

// maxBlurhash is the maximum width and height for a blurhash-rendered image. It
// doesn't have to be high resolution, since it's a blob of blur anyway.
const maxBlurhash = 25

func renderBlurhash(rawInfo json.RawMessage, w, h int, picFn func(*gdkpixbuf.Pixbuf)) {
	if rawInfo == nil {
		return
	}

	w, h = gotktrix.MaxSize(w, h, maxBlurhash, maxBlurhash)
	if w == 0 || h == 0 {
		return
	}

	var info struct {
		BlurHash string `json:"xyz.amorgan.blurhash"`
	}

	if err := json.Unmarshal(rawInfo, &info); err != nil || info.BlurHash == "" {
		return
	}

	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))

	if err := blurhash.DecodeDraw(nrgba, info.BlurHash, 1); err != nil {
		return
	}

	picFn(gdkpixbuf.NewPixbufFromBytes(
		glib.NewBytesWithGo(nrgba.Pix), gdkpixbuf.ColorspaceRGB, true, 8, w, h, nrgba.Stride,
	))
}
