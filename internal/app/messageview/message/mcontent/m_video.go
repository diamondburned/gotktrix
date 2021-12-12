package mcontent

import (
	"context"
	"log"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

type videoContent struct {
	gtk.Widgetter
	ctx          context.Context
	preview      *gtk.Picture
	thumbnailURL matrix.URL
}

var videoCSS = cssutil.Applier("mcontent-video", `
	.mcontent-video {
		padding: 0;
		margin:  0;
		margin-top: 6px;
	}
	.mcontent-videoplay {
		background-color: alpha(@theme_bg_color, 0.85);
		border-radius: 999px;
	}
	.mcontent-videoplay:hover,
	.mcontent-videoplay:active {
		background-color: @theme_selected_bg_color;
	}
`)

func newVideoContent(ctx context.Context, msg event.RoomMessageEvent) contentPart {
	client := gotktrix.FromContext(ctx).Offline()

	preview := gtk.NewPicture()
	preview.SetCanShrink(true)
	preview.SetCanFocus(false)
	preview.SetKeepAspectRatio(true)
	preview.SetHAlign(gtk.AlignStart)

	w := maxWidth
	h := maxHeight

	v, err := msg.VideoInfo()
	if err == nil {
		w, h = gotktrix.MaxSize(v.Width, v.Height, w, h)
		preview.SetSizeRequest(w, h)

		if v.Height > 0 && v.Width > 0 {
			renderBlurhash(msg.Info, w, h, preview.SetPixbuf)
		}
	}

	play := gtk.NewButtonFromIconName("media-playback-start-symbolic")
	play.SetHAlign(gtk.AlignCenter)
	play.SetVAlign(gtk.AlignCenter)
	play.AddCSSClass("mcontent-videoplay")

	ov := gtk.NewOverlay()
	ov.SetHAlign(gtk.AlignStart)
	ov.AddCSSClass("mcontent-video")
	ov.AddOverlay(play)
	ov.SetChild(preview)

	play.Connect("clicked", func() {
		u, err := client.MessageMediaURL(msg)
		if err != nil {
			log.Println("video URL error:", err)
			return
		}

		app.OpenURI(ctx, u)
	})

	return videoContent{
		Widgetter:    ov,
		ctx:          ctx,
		preview:      preview,
		thumbnailURL: v.ThumbnailURL,
	}
}

func (c videoContent) LoadMore() {
	pw, ph := c.preview.SizeRequest()
	client := gotktrix.FromContext(c.ctx)
	url, _ := client.ScaledThumbnail(c.thumbnailURL, pw, ph, gtkutil.ScaleFactor())
	imgutil.AsyncGET(c.ctx, url, c.preview.SetPaintable, imgutil.WithSizeOverrider(c.preview, pw, ph))
}

func (c videoContent) content() {}
