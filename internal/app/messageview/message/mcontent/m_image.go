package mcontent

import (
	"context"
	"encoding/json"
	"html"
	"image"
	"log"

	"github.com/bbrks/go-blurhash"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

type imageContent struct {
	*imageEmbed
	ctx context.Context
	msg *event.RoomMessageEvent
}

var imageCSS = cssutil.Applier("mcontent-image", `
	.mcontent-image-content {
		margin-top: 6px;
	}
	.mcontent-image {
		padding: 0;
		margin:  0;
		margin-left: -2px;
		border:  2px solid transparent;
		transition-duration: 150ms;
		transition-property: all;
	}
	.mcontent-image,
	.mcontent-image:hover {
		background: none;
	}
	.mcontent-image:hover {
		border: 2px solid @theme_selected_bg_color;
	}
	.mcontent-image > * {
		background-color: black;
		transition: linear 50ms filter;
	}
	.mcontent-image:hover > * {
		filter: contrast(80%) brightness(80%);
	}
	.mcontent-image-errorlabel {
		color: @error_color;
		padding: 4px;
	}
`)

func newImageContent(ctx context.Context, msg *event.RoomMessageEvent) *imageContent {
	embed := newImageEmbed(msg.Body, maxWidth, maxHeight)
	embed.AddCSSClass("mcontent-image-content")
	embed.whole = true
	embed.setOpenURL(func() {
		u, err := gotktrix.FromContext(ctx).MessageMediaURL(msg)
		if err != nil {
			log.Println("image URL error:", err)
			return
		}

		app.OpenURI(ctx, u)
	})

	c := imageContent{
		imageEmbed: embed,
		ctx:        ctx,
		msg:        msg,
	}

	i, err := msg.ImageInfo()
	if err == nil && i.Width > 0 && i.Height > 0 {
		c.setSize(i.Width, i.Height)
	} else {
		// Oversize and resize it back after.
		c.setSize(maxWidth, maxHeight)
	}

	return &c
}

func (c *imageContent) LoadMore() {
	if c.curSize != [2]int{} {
		renderBlurhash(c.msg.AdditionalInfo, c.curSize[0], c.curSize[1], c.image.SetPixbuf)
	}

	client := gotktrix.FromContext(c.ctx)
	url, _ := client.ImageThumbnail(c.msg, maxWidth, maxHeight, gtkutil.ScaleFactor())
	c.imageEmbed.useURL(c.ctx, url)
}

func (c *imageContent) content() {}

type imageEmbed struct {
	*gtk.Button
	image   *gtk.Picture
	openURL func()
	name    string
	curSize [2]int
	maxSize [2]int
	// whole, if true, will make errors show in its full information instead of
	// being hidden behind an error icon. Use this for messages only.
	whole bool
	// canHide, if true, will make the image hide itself on error. Use this for
	// anything not important, like embeds.
	canHide bool
}

func newImageEmbed(name string, maxW, maxH int) *imageEmbed {
	e := &imageEmbed{
		name:    name,
		maxSize: [2]int{maxW, maxH},
	}

	e.image = gtk.NewPicture()
	e.image.SetLayoutManager(gtk.NewConstraintLayout()) // magically left aligned
	e.image.SetCanFocus(false)
	e.image.SetCanShrink(true)
	e.image.SetKeepAspectRatio(true)

	e.Button = gtk.NewButton()
	e.Button.SetOverflow(gtk.OverflowHidden)
	e.Button.SetHAlign(gtk.AlignStart)
	e.Button.SetHasFrame(false)
	e.Button.SetChild(e.image)
	e.Button.SetTooltipText(name)
	e.Button.SetSensitive(false)
	e.Button.ConnectClicked(func() { e.openURL() })
	imageCSS(e.Button)

	return e
}

func (e *imageEmbed) useURL(ctx context.Context, url string) {
	// Only load the image when we actually draw the image.
	gtkutil.OnFirstDraw(e, func() {
		imgutil.AsyncGET(ctx, url, e.setPaintable, imgutil.WithErrorFn(e.onError))
	})
}

func (e *imageEmbed) setPaintable(p gdk.Paintabler) {
	e.setSize(p.IntrinsicWidth(), p.IntrinsicHeight())
	e.image.SetPaintable(p)
	e.image.QueueResize()

	// undo effects

	if e.canHide {
		e.Show()
	}
	if e.whole {
		e.Button.SetChild(e.image)
	}
}

func (e *imageEmbed) onError(err error) {
	if e.canHide {
		e.Hide()
		return
	}

	if e.whole {
		// Mild annoyance: the padding of this label actually grows the image a
		// bit. Not sure how to fix it.
		errLabel := gtk.NewLabel("Error fetching image: " + html.EscapeString(err.Error()))
		errLabel.AddCSSClass("mcontent-image-errorlabel")
		errLabel.SetEllipsize(pango.EllipsizeEnd)
		errLabel.SetWrap(true)
		errLabel.SetWrapMode(pango.WrapWordChar)
		errLabel.SetLines(2)
		e.Button.SetChild(errLabel)
	} else {
		size := e.curSize
		if size == [2]int{} {
			// No size; pick the max size.
			size = e.maxSize
		}
		iconMissing := imgutil.IconPaintable("image-missing", size[0], size[1])
		e.image.SetPaintable(iconMissing)
	}

	var tooltip string
	if e.name != "" {
		tooltip += html.EscapeString(e.name) + "\n"
	}
	tooltip += "<b>Error:</b> " + html.EscapeString(err.Error())
	e.Button.SetTooltipMarkup(tooltip)
}

func (e *imageEmbed) setOpenURL(f func()) {
	e.openURL = f
	e.Button.SetSensitive(f != nil)
}

func (e *imageEmbed) setSize(w, h int) {
	w, h = gotktrix.MaxSize(w, h, e.maxSize[0], e.maxSize[1])
	e.curSize = [2]int{w, h}
	e.SetSizeRequest(w, h)
}

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
