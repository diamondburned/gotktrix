package mcontent

import (
	"context"
	_ "embed"
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/mediautil"
	"github.com/diamondburned/gotrix/event"
)

type videoContent struct {
	gtk.Widgetter
	ctx     context.Context
	preview *gtk.Picture

	thumbURL string
	url      string
	size     [2]int
}

//go:embed styles/mcontent-video.css
var videoStyle string
var videoCSS = cssutil.Applier("mcontent-video", videoStyle)

func newVideoContent(ctx context.Context, msg *event.RoomMessageEvent) contentPart {
	client := gotktrix.FromContext(ctx).Offline()

	preview := gtk.NewPicture()
	preview.AddCSSClass("mcontent-video-preview")
	preview.SetLayoutManager(gtk.NewConstraintLayout()) // magically left aligned
	preview.SetCanShrink(true)
	preview.SetCanFocus(false)
	preview.SetKeepAspectRatio(true)
	preview.SetHAlign(gtk.AlignStart)

	w := maxWidth
	h := maxHeight

	var thumbnailURL string

	v, err := msg.VideoInfo()
	if err == nil {
		w, h = gotktrix.MaxSize(v.Width, v.Height, w, h)
		if v.Height > 0 && v.Width > 0 {
			renderBlurhash(msg.AdditionalInfo, w, h, preview.SetPixbuf)
		}
		if v.ThumbnailURL != "" {
			thumbnailURL, _ = client.ScaledThumbnail(v.ThumbnailURL, w, h, gtkutil.ScaleFactor())
		}
	}

	preview.SetSizeRequest(w, h)

	playIcon := gtk.NewImageFromIconName("media-playback-start-symbolic")
	playIcon.AddCSSClass("mcontent-video-play-icon")
	playIcon.SetHAlign(gtk.AlignCenter)
	playIcon.SetVAlign(gtk.AlignCenter)
	playIcon.SetIconSize(gtk.IconSizeLarge)

	ov := gtk.NewOverlay()
	ov.SetHAlign(gtk.AlignStart)
	ov.AddOverlay(playIcon)
	ov.SetChild(preview)

	play := gtk.NewButtonFromIconName("media-playback-start-symbolic")
	play.SetHAlign(gtk.AlignStart)
	play.SetOverflow(gtk.OverflowHidden)
	play.SetHasFrame(false)
	play.SetTooltipText(msg.Body)
	play.SetChild(ov)
	videoCSS(play)

	url, urlErr := client.MessageMediaURL(msg)

	play.ConnectClicked(func() {
		if urlErr != nil {
			app.Error(ctx, urlErr)
			return
		}
		app.OpenURI(ctx, url)
	})

	return videoContent{
		Widgetter: play,
		ctx:       ctx,
		preview:   preview,
		thumbURL:  thumbnailURL,
		url:       url,
		size:      [2]int{w, h},
	}
}

func (c videoContent) LoadMore() {
	if c.thumbURL != "" {
		imgutil.AsyncGET(c.ctx, c.thumbURL, c.preview.SetPaintable)
		return
	}

	if c.url == "" {
		return
	}

	gtkutil.Async(c.ctx, func() func() {
		p, err := mediautil.Thumbnail(c.ctx, c.url, c.size[0], c.size[1])
		if err != nil {
			log.Println("ffmpeg thumbnail error:", err)
			return nil
		}

		if p == "" {
			return nil
		}

		return func() {
			c.preview.SetFilename(p)
		}
	})
}

func (c videoContent) content() {}
