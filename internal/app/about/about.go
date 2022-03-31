package about

import (
	"context"
	"path"
	"runtime/debug"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
)

// Show shows the about dialog.
func Show(ctx context.Context) *gtk.AboutDialog {
	// TODO: add go.mod parsing for authors maybe

	about := gtk.NewAboutDialog()
	about.SetTransientFor(app.GTKWindowFromContext(ctx))
	about.SetModal(true)
	about.SetProgramName("gotktrix")
	about.SetVersion("git") // TODO version
	about.SetWebsite("https://github.com/diamondburned/gotktrix")
	about.SetWebsiteLabel("Source code")
	about.SetLicenseType(gtk.LicenseAGPL30)
	about.SetAuthors([]string{
		"diamondburned",
		"chanbakjsd (library)",
	})

	about.AddCreditSection("Sound Files", []string{
		// https://directory.fsf.org/wiki/Sound-theme-freedesktop
		"freedesktop.org",
		"Lennart Poettering",
	})

	build, ok := debug.ReadBuildInfo()
	if !ok {
		panic("gotktrix not build with module support")
	}

	about.AddCreditSection("Dependency Authors", modAuthors(build.Deps))

	about.Show()

	return about
}

func modAuthors(mods []*debug.Module) []string {
	authors := make([]string, 0, len(mods))
	authMap := make(map[string]struct{}, len(mods))

	for _, mod := range mods {
		author := path.Dir(mod.Path)
		if _, ok := authMap[author]; !ok {
			authors = append(authors, author)
			authMap[author] = struct{}{}
		}
	}

	return authors
}
