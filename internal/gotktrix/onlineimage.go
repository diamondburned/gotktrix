package gotktrix

import (
	"context"
	"net/url"

	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

type mxcProvider struct {
	Width  int
	Height int
	Flags  ImageFlags
}

// ImageFlags is describes boolean attributes for fetching Matrix images.
type ImageFlags uint8

const (
	// ImageNormal is the 0 flag.
	ImageNormal ImageFlags = 0
	// MatrixNoCrop asks the server to scale the image down to fit the frame
	// instead of cropping the image.
	ImageNoCrop ImageFlags = 1 << (iota - 1)
	// ImageSkip1xScale skips the 1x scale factor. This is useful if the
	// specified image size is large enough for either 1x or 2x, since it works
	// better with the image cache.
	ImageSkip1xScale
)

// Has returns true if f has this.
func (f ImageFlags) Has(this ImageFlags) bool {
	return f&this == this
}

// MXCProvider returns a new universal resource provider that handles MXC URLs.
func MXCProvider(w, h int, flags ImageFlags) imgutil.Provider {
	return mxcProvider{w, h, flags}
}

// AvatarProvider is the image provider that all avatar widgets should use.
var AvatarProvider = imgutil.NewProviders(
	imgutil.HTTPProvider,
	MXCProvider(128, 128, ImageNormal|ImageSkip1xScale),
)

// Schemes implements Provider.
func (p mxcProvider) Schemes() []string {
	return []string{"mxc"}
}

// AsyncDo implements Provider.
func (p mxcProvider) Do(ctx context.Context, url *url.URL, img imgutil.ImageSetter) {
	client := FromContext(ctx)
	if client == nil {
		imgutil.OptsError(ctx, errors.New("context missing gotktrix.Client"))
		return
	}

	w := p.Width
	h := p.Height
	s := gtkutil.ScaleFactor()

	switch s {
	case 0:
		return
	case 1:
		if p.Flags.Has(ImageSkip1xScale) {
			s = 2
		}
	}

	w *= s
	h *= s

	mxc := url.String()

	var str string
	if p.Flags.Has(ImageNoCrop) {
		str, _ = client.ScaledThumbnail(matrix.URL(mxc), w, h, s)
	} else {
		str, _ = client.Thumbnail(matrix.URL(mxc), w, h, s)
	}

	if str == "" {
		return
	}

	imgutil.AsyncGET(ctx, str, img)
}
