package mcontent

import (
	"context"
	"log"
	"mime"
	"time"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/mediautil"
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
	v, err := msg.VideoInfo()
	if err != nil {
		return newErroneousContent(err.Error(), -1, -1)
	}

	w, h := mediautil.MaxSize(v.Width, v.Height, maxWidth, maxHeight)

	play := gtk.NewButtonFromIconName("media-playback-start-symbolic")
	play.SetHAlign(gtk.AlignCenter)
	play.SetVAlign(gtk.AlignCenter)
	play.AddCSSClass("mcontent-videoplay")

	client := gotktrix.FromContext(ctx).Offline()

	var fetched bool

	thumbnail := gtk.NewImage()
	thumbnail.SetSizeRequest(w, h)
	thumbnail.Connect("map", func() {
		if !fetched {
			url, _ := client.Thumbnail(v.ThumbnailURL, w, h)
			imgutil.AsyncGET(ctx, url, thumbnail.SetFromPaintable)
			fetched = true
		}
	})

	ov := gtk.NewOverlay()
	ov.SetHAlign(gtk.AlignStart)
	ov.AddOverlay(play)
	ov.SetChild(thumbnail)

	bin := adw.NewBin()
	bin.SetChild(ov)
	videoCSS(bin)

	play.Connect("clicked", func() {
		filename := msg.Body

		if filename == "" {
			t, err := mime.ExtensionsByType(v.MimeType)
			if err == nil && t != nil {
				filename = "video" + t[0]
			}
		}

		u, err := client.MediaDownloadURL(msg.URL, true, filename)
		if err != nil {
			log.Println("video URL error:", err)
			return
		}

		ts := uint32(time.Now().Unix())
		gtk.ShowURI(app.FromContext(ctx).Window(), u, ts)

		/*
			url, e := client.MediaDownloadURL(msg.URL, true, "")
			if e != nil {
				log.Println("failed to get media URL:", err)
				return
			}

			log.Println("got video URL", url)
			stream := mediautil.Stream(ctx, url)

			v := gtk.NewVideoForMediaStream(stream)
			v.SetAutoplay(true)
			bin.SetChild(v)

			// The button is no longer needed anymore.
			play.SetSensitive(false)
			play.Unparent()
		*/
	})

	return videoContent{
		Bin: bin,
	}
}

func (c videoContent) content() {}
