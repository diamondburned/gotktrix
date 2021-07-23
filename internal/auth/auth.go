// Package auth supplies a gtk.Assistant wrapper to provide a login screen.
package auth

import (
	"github.com/chanbakjsd/gotrix/api/httputil"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
)

type Assistant struct {
	*assistant.Assistant
	client   httputil.Client
	accounts []account
}

type discoverStep struct {
	// states
	serverName string
}

// New creates a new authentication assistant with the default HTTP client.
func New(parent *gtk.Window) *Assistant {
	return NewWithClient(parent, httputil.NewClient())
}

// NewWithClient creates a new authentication assistant with the given HTTP
// client.
func NewWithClient(parent *gtk.Window, client httputil.Client) *Assistant {
	ass := assistant.New(parent, nil)
	ass.SetTitle("Getting Started")
	ass.Busy()

}

func (a *Assistant) createServerNamePage() gtk.Widgetter {}

func makeInputs(names ...string) (gtk.Widgetter, []*gtk.Entry) {
	entries := make()
}
