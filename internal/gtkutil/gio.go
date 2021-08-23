package gtkutil

import (
	"context"
	"io"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
)

// WrapInputStream wraps gio.InputStream to satisfy io.ReadCloser.
func WrapInputStream(ctx context.Context, in gio.InputStreamer) io.ReadCloser {
	return gioutil.Reader(ctx, in)
}
