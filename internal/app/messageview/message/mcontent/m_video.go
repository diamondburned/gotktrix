package mcontent

import (
	"context"
	"log"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

type videoContent struct {
	*adw.Bin
}

var videoCSS = cssutil.Applier("mcontent-video", `
	.mcontent-video > overlay > image {
		background-color: black;
	}
	.mcontent-videoplay {
		background-color: alpha(@theme_bg_color, 0.85);
		border-radius: 999px;
	}
	.mcontent-videoplay:hover,
	.mcontent-videoplay:active {
		background-color: @accent_bg_color;
	}
`)

func newVideoContent(ctx context.Context, msg event.RoomMessageEvent) contentPart {
	client := gotktrix.FromContext(ctx).Offline()

	var fetched bool

	preview := gtk.NewPicture()
	preview.SetSizeRequest(100, 100)
	preview.SetCanShrink(true)
	preview.SetKeepAspectRatio(true)

	w := maxWidth * thumbnailScale
	h := maxHeight * thumbnailScale

	v, err := msg.VideoInfo()
	if err == nil {
		w, h = gotktrix.MaxSize(v.Width, v.Height, w, h)

		// Recalcualte the max dimensions without scaling.
		_, actualHeight := gotktrix.MaxSize(v.Width, v.Height, maxWidth, maxHeight)
		preview.SetSizeRequest(100, actualHeight)
	}

	if w > 0 && h > 0 {
		if blur := renderBlurhash(msg.Info, w, h); blur != nil {
			preview.SetPaintable(blur)
		}
	}

	preview.Connect("map", func() {
		if !fetched {
			fetched = true

			url, _ := client.ScaledThumbnail(v.ThumbnailURL, w, h)
			imgutil.AsyncGET(ctx, url, preview.SetPaintable)
		}
	})

	play := gtk.NewButtonFromIconName("media-playback-start-symbolic")
	play.SetHAlign(gtk.AlignCenter)
	play.SetVAlign(gtk.AlignCenter)
	play.AddCSSClass("mcontent-videoplay")

	ov := gtk.NewOverlay()
	ov.AddOverlay(play)
	ov.SetChild(preview)

	bin := adw.NewBin()
	bin.SetHAlign(gtk.AlignStart)
	bin.SetChild(ov)
	videoCSS(bin)

	play.Connect("clicked", func() {
		u, err := client.MessageMediaURL(msg)
		if err != nil {
			log.Println("video URL error:", err)
			return
		}

		app.OpenURI(ctx, u)
	})

	return videoContent{
		Bin: bin,
	}
}

func (c videoContent) content() {}
