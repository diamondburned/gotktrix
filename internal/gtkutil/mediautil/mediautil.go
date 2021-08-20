package mediautil

import (
	"context"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/pkg/errors"
)

// Client is the HTTP client used to fetch all images.
var Client = http.Client{
	Timeout: 4 * time.Minute,
	Transport: httpcache.NewTransport(
		diskcache.New(config.CacheDir("media")),
	),
}

// MIME tries to get the MIME type off a seekable reader. The reader is seeked
// back to before bytes were read.
func MIME(f io.ReadSeeker) string {
	buf := make([]byte, 512)

	n, err := f.Read(buf)
	if err != nil {
		return ""
	}

	defer f.Seek(-int64(n), io.SeekCurrent)

	typ := http.DetectContentType(buf)
	// Trim the charset stuff off.
	mime, _, _ := mime.ParseMediaType(typ)
	return mime
}

// Stream creates a media streamer that asynchronously fetches the given URL.
//
// This function currently panics, because stream playback of GtkVideo is
// currently not implemented in their source code. How funny.
func Stream(ctx context.Context, url string) gtk.MediaStreamer {
	if url == "" {
		return nil
	}

	rp, wp, err := os.Pipe()
	if err != nil {
		log.Println("mediautil: failed to mkpipe:", err)
		return nil
	}

	istream := gio.NewUnixInputStream(int(rp.Fd()), true)

	mfile := gtk.NewMediaFileForInputStream(istream)
	// Must keep the read file descriptor alive.
	mfile.Connect("notify::ended", func() {
		log.Println("stream ended:", url)
		_ = rp
	})

	onErr := func(err error) {
		glib.IdleAddPriority(glib.PriorityHigh, func() { mfile.GError(err) })
	}

	go func() {
		defer wp.Close()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			onErr(errors.Wrapf(err, "failed to create request %q", url))
			return
		}

		r, err := Client.Do(req)
		if err != nil {
			onErr(errors.Wrapf(err, "failed to GET %q", url))
			return
		}
		defer r.Body.Close()

		_, err = io.Copy(wp, r.Body)
		if err != nil {
			onErr(errors.Wrap(err, "error copying to gio.UnixInputStream"))
		}
	}()

	return mfile
}

func init() {
	mime.AddExtensionType(".3gp", "video/3gpp")
	mime.AddExtensionType(".3gpp", "video/3gpp")
	mime.AddExtensionType(".3g2", "video/3gpp2")
	mime.AddExtensionType(".m3u8", "application/x-mpegURL")
	mime.AddExtensionType(".h261", "video/h261")
	mime.AddExtensionType(".h263", "video/h263")
	mime.AddExtensionType(".h264", "video/h264")
	mime.AddExtensionType(".jpgv", "video/jpeg")
	mime.AddExtensionType(".jpm", "video/jpm")
	mime.AddExtensionType(".jgpm", "video/jpm")
	mime.AddExtensionType(".mj2", "video/mj2")
	mime.AddExtensionType(".mjp2", "video/mj2")
	mime.AddExtensionType(".ts", "video/mp2t")
	mime.AddExtensionType(".mp4", "video/mp4")
	mime.AddExtensionType(".mp4v", "video/mp4")
	mime.AddExtensionType(".mpg4", "video/mp4")
	mime.AddExtensionType(".mpeg", "video/mpeg")
	mime.AddExtensionType(".mpg", "video/mpeg")
	mime.AddExtensionType(".mpe", "video/mpeg")
	mime.AddExtensionType(".m1v", "video/mpeg")
	mime.AddExtensionType(".m2v", "video/mpeg")
	mime.AddExtensionType(".ogv", "video/ogg")
	mime.AddExtensionType(".qt", "video/quicktime")
	mime.AddExtensionType(".mov", "video/quicktime")
	mime.AddExtensionType(".uvh", "video/vnd.dece.hd")
	mime.AddExtensionType(".uvvh", "video/vnd.dece.hd")
	mime.AddExtensionType(".uvm", "video/vnd.dece.mobile")
	mime.AddExtensionType(".uvvm", "video/vnd.dece.mobile")
	mime.AddExtensionType(".uvp", "video/vnd.dece.pd")
	mime.AddExtensionType(".uvvp", "video/vnd.dece.pd")
	mime.AddExtensionType(".uvs", "video/vnd.dece.sd")
	mime.AddExtensionType(".uvvs", "video/vnd.dece.sd")
	mime.AddExtensionType(".uvv", "video/vnd.dece.video")
	mime.AddExtensionType(".uvvv", "video/vnd.dece.video")
	mime.AddExtensionType(".dvb", "video/vnd.dvb.file")
	mime.AddExtensionType(".fvt", "video/vnd.fvt")
	mime.AddExtensionType(".mxu", "video/vnd.mpegurl")
	mime.AddExtensionType(".m4u", "video/vnd.mpegurl")
	mime.AddExtensionType(".pyv", "video/vnd.ms-playready.media.pyv")
	mime.AddExtensionType(".uvu", "video/vnd.uvvu.mp4")
	mime.AddExtensionType(".uvvu", "video/vnd.uvvu.mp4")
	mime.AddExtensionType(".viv", "video/vnd.vivo")
	mime.AddExtensionType(".webm", "video/webm")
	mime.AddExtensionType(".f4v", "video/x-f4v")
	mime.AddExtensionType(".fli", "video/x-fli")
	mime.AddExtensionType(".flv", "video/x-flv")
	mime.AddExtensionType(".m4v", "video/x-m4v")
	mime.AddExtensionType(".mkv", "video/x-matroska")
	mime.AddExtensionType(".mk3d", "video/x-matroska")
	mime.AddExtensionType(".mks", "video/x-matroska")
	mime.AddExtensionType(".mng", "video/x-mng")
	mime.AddExtensionType(".asf", "video/x-ms-asf")
	mime.AddExtensionType(".asx", "video/x-ms-asf")
	mime.AddExtensionType(".vob", "video/x-ms-vob")
	mime.AddExtensionType(".wm", "video/x-ms-wm")
	mime.AddExtensionType(".wmv", "video/x-ms-wmv")
	mime.AddExtensionType(".wmx", "video/x-ms-wmx")
	mime.AddExtensionType(".wvx", "video/x-ms-wvx")
	mime.AddExtensionType(".avi", "video/x-msvideo")
	mime.AddExtensionType(".movie", "video/x-sgi-movie")
	mime.AddExtensionType(".smv", "video/x-smv")
}
