package mediautil

import (
	"context"
	"io"
	"mime"
	"net/http"

	"github.com/diamondburned/gotk4/pkg/gio/v2"

	gioglib "github.com/diamondburned/gotk4/pkg/glib/v2"
)

// MIME tries to get the MIME type off a seekable reader. The reader is seeked
// back to before bytes were read.
func MIME(f io.ReadSeeker) string {
	buf := make([]byte, 512)

	n, err := f.Read(buf)
	if err != nil {
		return ""
	}

	f.Seek(0, io.SeekStart)

	return detectCT(buf[:n])
}

// FileMIME tries to get the MIME type of the given GIO file.
func FileMIME(ctx context.Context, f *gio.FileInputStream) string {
	// By the end of this function, ensure that any consumed read is undone.
	defer f.Seek(ctx, 0, gioglib.SeekSet)

	info, err := f.QueryInfo(ctx, gio.FILE_ATTRIBUTE_STANDARD_CONTENT_TYPE)
	if err == nil {
		if mime := gio.ContentTypeGetMIMEType(info.ContentType()); mime != "" {
			return mime
		}
	}

	buf := make([]byte, 512)

	n, err := f.Read(ctx, buf)
	if err != nil {
		return ""
	}

	return detectCT(buf[:n])
}

func detectCT(b []byte) string {
	typ := http.DetectContentType(b)
	// Trim the charset stuff off.
	mime, _, _ := mime.ParseMediaType(typ)
	return mime
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
