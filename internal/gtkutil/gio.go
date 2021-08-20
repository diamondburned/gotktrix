package gtkutil

import (
	"context"
	"io"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
)

// WrapInputStream wraps gio.InputStream to satisfy io.ReadCloser.
func WrapInputStream(ctx context.Context, in gio.InputStreamer) io.ReadCloser {
	return gioReader{ctx, in}
}

type gioReader struct {
	ctx context.Context
	r   gio.InputStreamer
}

func (r gioReader) Read(b []byte) (int, error) {
	i, err := r.r.Read(r.ctx, b)
	if err != nil {
		return i, err
	}

	if i == 0 {
		return 0, io.EOF
	}

	return i, nil
}

func (r gioReader) Close() error {
	return r.r.Close(r.ctx)
}
