package mcontent

import (
	"context"
	"encoding/json"
	"image"
	"log"

	"github.com/bbrks/go-blurhash"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
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

	button *gtk.Button
	layout *gtk.ConstraintLayout
	image  *gtk.Picture
	size   [2]int
}

var imageCSS = cssutil.Applier("mcontent-image", `
	.mcontent-image {
		padding: 0;
		margin:  0;
		margin-top: 6px;
	}
`)

func newImageContent(ctx context.Context, msg event.RoomMessageEvent) contentPart {
	c := imageContent{
		ctx: ctx,
		msg: msg,
	}

	c.layout = gtk.NewConstraintLayout()

	c.image = gtk.NewPicture()
	c.image.SetLayoutManager(c.layout)
	c.image.SetCanFocus(false)
	c.image.SetCanShrink(true)
	c.image.SetKeepAspectRatio(true)

	c.button = gtk.NewButton()
	c.button.AddCSSClass("mcontent-image")
	c.button.SetHAlign(gtk.AlignStart)
	c.button.SetHasFrame(false)
	c.button.SetChild(c.image)
	c.button.SetTooltipText(msg.Body)
	c.button.Connect("clicked", func() {
		u, err := gotktrix.FromContext(ctx).MessageMediaURL(msg)
		if err != nil {
			log.Println("image URL error:", err)
			return
		}

		app.OpenURI(ctx, u)
	})

	i, err := msg.ImageInfo()
	if err == nil && i.Width > 0 && i.Height > 0 {
		w, h := gotktrix.MaxSize(i.Width, i.Height, maxWidth, maxHeight)
		c.setSize(w, h)
		renderBlurhash(msg.Info, w, h, c.image.SetPixbuf)
	}

	c.Widgetter = c.button

	// 	box := gtk.NewBox(gtk.OrientationVertical, 0)
	// 	box.SetHExpand(false)
	// 	box.Append(button)

	return &c
}

func (c *imageContent) LoadMore() {
	client := gotktrix.FromContext(c.ctx)
	url, _ := client.ImageThumbnail(c.msg, maxWidth, maxHeight, gtkutil.ScaleFactor())

	imgutil.AsyncGET(c.ctx, url, func(p gdk.Paintabler) {
		if c.size == [2]int{} {
			c.setSize(gotktrix.MaxSize(
				p.IntrinsicWidth(), p.IntrinsicHeight(),
				maxWidth, maxHeight,
			))
		}
		c.image.SetPaintable(p)
	})
}

func (c *imageContent) setSize(w, h int) {
	c.size = [2]int{w, h}
	c.image.SetSizeRequest(w, h)

	/*
		guide := gtk.NewConstraintGuide()
		guide.SetMinSize(gotktrix.MaxSize(w, h, 64, 64))
		guide.SetNatSize(w, h)
		guide.SetMaxSize(w, h)

		c.layout.RemoveAllConstraints()
		c.layout.AddGuide(guide)
		c.layout.AddConstraint(gtk.NewConstraint(
			nil, gtk.ConstraintAttributeHeight, gtk.ConstraintRelationEq,
			nil, gtk.ConstraintAttributeWidth, float64(h)/float64(w), 0,
			int(gtk.ConstraintStrengthRequired),
		))
		c.layout.AddConstraint(gtk.NewConstraint(
			nil, gtk.ConstraintAttributeWidth, gtk.ConstraintRelationEq,
			guide, gtk.ConstraintAttributeWidth, 1, 0,
			int(gtk.ConstraintStrengthRequired),
		))
		c.layout.AddConstraint(gtk.NewConstraint(
			nil, gtk.ConstraintAttributeHeight, gtk.ConstraintRelationEq,
			guide, gtk.ConstraintAttributeHeight, 1, 0,
			int(gtk.ConstraintStrengthRequired),
		))

		c.image.QueueResize()
	*/
}

func (c *imageContent) content() {}

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
