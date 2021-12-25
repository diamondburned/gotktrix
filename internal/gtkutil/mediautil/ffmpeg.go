package mediautil

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/pkg/errors"
)

var thumbnailDir = config.CacheDir("thumbnail")

// ThumbnailFormat is the output format for the thumbnail.
const ThumbnailFormat = "jpeg"

var hasFFmpeg bool

func init() {
	ffmpeg, _ := exec.LookPath("ffmpeg")
	hasFFmpeg = ffmpeg != ""
}

var thumbnailGC = cacheGC{
	Dir: thumbnailDir,
	Age: 24 * time.Hour,
}

// Thumbnail fetches the thumbnail of the given URL and returns the path to the
// file.
func Thumbnail(ctx context.Context, url string, w, h int) (string, error) {
	if !hasFFmpeg {
		return "", nil
	}

	return doTmp(
		thumbnailURLPath(url, fmt.Sprintf("w=%d;h=%d", w, h)),
		func(out string) error {
			thumbnailGC.do()
			out = out + "." + ThumbnailFormat // add this into the temp path
			return doFFmpeg(ctx, url, out, "-frames:v", "1", "-f", "image2")
		},
	)
}

func thumbnailURLPath(url, fragment string) string {
	b := sha1.Sum([]byte(url + "#" + fragment))
	f := base64.StdEncoding.EncodeToString(b[:])
	return filepath.Join(thumbnailDir, f)
}

func doFFmpeg(ctx context.Context, src, dst string, opts ...string) error {
	args := make([]string, 0, len(opts)+10)
	args = append(args, "-y", "-loglevel", "error", "-i", src)
	args = append(args, opts...)
	args = append(args, dst)

	if err := exec.CommandContext(ctx, "ffmpeg", args...).Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			log.Println("ffmpeg error:", string(exitErr.Stderr))
		}

		return err
	}

	thumbnailGC.do()

	return nil
}
