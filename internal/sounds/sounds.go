package sounds

import (
	"embed"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/pkg/errors"
)

//go:embed *.opus
var sounds embed.FS

// Sound IDs.
const (
	Bell    = "bell.opus"
	Message = "message.opus"
)

type mediaFile struct {
	*gtk.MediaFile
	Error error
}

var mediaFiles = map[string]mediaFile{}

// Play plays the given sound ID. It first uses Canberra, falling back to
// ~/.cache/gotktrix/{id}.opus, then the embedded audio (if any), then
// display.Beep() otherwise.
func Play(id string) {
	canberra := exec.Command("canberra-gtk-play", "--id", id)
	if err := canberra.Run(); err == nil {
		return
	} else {
		log.Println("canberra error:", err)
	}

	if filepath.Ext(id) == "" {
		id += ".opus"
	}

	media, ok := mediaFiles[id]
	if ok {
		glib.IdleAdd(func() {
			if media.Error != nil {
				playEmbedError(id, media.Error)
			} else {
				media.Play()
			}
		})
		return
	}

	dst := config.CacheDir("sounds", id)

	_, err := os.Stat(dst)
	if err != nil {
		if err := copyToFS(dst, id); err != nil {
			log.Printf("cannot copy sound %q: %v", id, err)
			glib.IdleAdd(beep)
			return
		}
	}

	glib.IdleAdd(func() {
		media := gtk.NewMediaFileForFilename(dst)
		mediaFiles[id] = mediaFile{
			MediaFile: media,
			Error:     nil,
		}

		media.Connect("notify::error", func() {
			f := mediaFiles[id]
			f.Error = media.Error()
			mediaFiles[id] = f

			playEmbedError(id, f.Error)
		})

		media.Play()
	})
}

func playEmbedError(name string, err error) {
	log.Printf("error playing embedded %s.opus: %v", name, err)
	beep()
}

func beep() {
	log.Println("using beep() instead")
	disp := gdk.DisplayGetDefault()
	disp.Beep()
}

func copyToFS(dst string, name string) error {
	src, err := sounds.Open(name)
	if err != nil {
		return err
	}

	defer src.Close()

	dir := filepath.Dir(dst)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Wrap(err, "cannot mkdir sounds/")
	}

	f, err := os.CreateTemp(dir, ".tmp.*")
	if err != nil {
		return errors.Wrap(err, "cannot mktemp in cache dir")
	}

	defer os.Remove(f.Name())
	defer f.Close()

	if _, err := io.Copy(f, src); err != nil {
		return errors.Wrap(err, "cannot write audio to disk")
	}

	if err := f.Close(); err != nil {
		return errors.Wrap(err, "cannot close written audio")
	}

	if err := os.Rename(f.Name(), dst); err != nil {
		return errors.Wrap(err, "cannot commit written audio")
	}

	return nil
}
