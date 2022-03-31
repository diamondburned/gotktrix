package mediautil

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/diamondburned/gotkit/app"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

// thumbnailTmpPattern has the output format for the thumbnail.
const thumbnailTmpPattern = "*.jpeg"

var hasFFmpeg bool

func init() {
	ffmpeg, _ := exec.LookPath("ffmpeg")
	hasFFmpeg = ffmpeg != ""
}

// Thumbnail fetches the thumbnail of the given URL and returns the path to the
// file.
func Thumbnail(ctx context.Context, url string, w, h int) (string, error) {
	if !hasFFmpeg {
		return "", nil
	}

	app := app.FromContext(ctx)
	thumbDir := app.CachePath("thumbnails")

	return doTmp(
		thumbnailURLPath(thumbDir, url, fmt.Sprintf("w=%d;h=%d", w, h)),
		thumbnailTmpPattern,
		func(out string) error {
			doGC(thumbDir, 24*time.Hour)
			return doFFmpeg(ctx, url, out, "-frames:v", "1", "-f", "image2")
		},
	)
}

func thumbnailURLPath(baseDir, url, fragment string) string {
	b := sha1.Sum([]byte(url + "#" + fragment))
	f := base64.URLEncoding.EncodeToString(b[:])
	return filepath.Join(baseDir, f)
}

var ffmpegSema = semaphore.NewWeighted(int64(runtime.GOMAXPROCS(-1)))

func doFFmpeg(ctx context.Context, src, dst string, opts ...string) error {
	if err := ffmpegSema.Acquire(ctx, 1); err != nil {
		return err
	}
	defer ffmpegSema.Release(1)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	args := make([]string, 0, len(opts)+10)
	args = append(args, "-y", "-loglevel", "warning", "-i", src)
	args = append(args, opts...)
	args = append(args, dst)

	if err := exec.CommandContext(ctx, "ffmpeg", args...).Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			log.Println("ffmpeg error:", string(exitErr.Stderr))
		}

		return err
	}

	return nil
}
