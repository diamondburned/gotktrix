package mauthor

import (
	"fmt"
	"hash"
	"hash/fnv"
	"image/color"
	"math"
	"sync"
)

// ColorHasher describes a string hasher that outputs a color.
type ColorHasher interface {
	Hash(name string) color.RGBA
}

// darkThreshold is DarkColorHasher's value.
const darkThreshold = 0.65

var (
	// FNVHasher is the string hasher used for color hashing.
	FNVHasher = func() hash.Hash32 { return fnv.New32a() }

	// LightColorHasher generates a pastel color name for use with a dark
	// background.
	LightColorHasher ColorHasher = HSVHasher{FNVHasher, 0.32, 0.97}
	// DarkColorHasher generates a darker, stronger color name for use with a
	// light background.
	DarkColorHasher ColorHasher = HSVHasher{FNVHasher, 1.00, 0.65}
)

// RGBHex converts the given color to a HTML hex color string. The alpha value
// is ignored.
func RGBHex(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// rgbIsDark determines if the given RGB colors are dark or not. It takes in
// colors of range [0.0, 1.0].
func rgbIsDark(r, g, b float64) bool {
	// Determine the value in the HSV colorspace. Code taken from
	// lucasb-eyer/go-colorful.
	v := math.Max(math.Max(r, g), b)
	return v <= darkThreshold
}

// HSVHasher describes a color hasher that accepts saturation and value
// parameters in the HSV color space.
type HSVHasher struct {
	H func() hash.Hash32 // hashing function
	S float64            // saturation
	V float64            // value
}

const nColors = 32 // hue count

// Hash hashes the given name using the parameters inside HSVHasher.
func (h HSVHasher) Hash(name string) color.RGBA {
	hash := h.H()
	hash.Write([]byte(name))

	hue := float64(hash.Sum32()%nColors) * 360 / nColors
	return hsvrgb(hue, h.S, h.V)
}

// hsvrgb is taken from lucasb-eyer/go-colorful, licensed under the MIT license.
func hsvrgb(h, s, v float64) color.RGBA {
	Hp := h / 60.0
	C := v * s
	X := C * (1.0 - math.Abs(math.Mod(Hp, 2.0)-1.0))

	m := v - C
	r, g, b := 0.0, 0.0, 0.0

	switch {
	case 0.0 <= Hp && Hp < 1.0:
		r = C
		g = X
	case 1.0 <= Hp && Hp < 2.0:
		r = X
		g = C
	case 2.0 <= Hp && Hp < 3.0:
		g = C
		b = X
	case 3.0 <= Hp && Hp < 4.0:
		g = X
		b = C
	case 4.0 <= Hp && Hp < 5.0:
		r = X
		b = C
	case 5.0 <= Hp && Hp < 6.0:
		r = C
		b = X
	}

	return color.RGBA{
		R: uint8((m + r) * 0xFF),
		G: uint8((m + g) * 0xFF),
		B: uint8((m + b) * 0xFF),
		A: 0xFF,
	}
}

var (
	defaultHasher = LightColorHasher
	hasherMutex   sync.RWMutex
)

// DefaultColorHasher returns the default color hasher.
func DefaultColorHasher() ColorHasher {
	hasherMutex.RLock()
	defer hasherMutex.RUnlock()

	return defaultHasher
}

// SetDefaultColorHasher sets the default hasher that the package uses.
func SetDefaultColorHasher(hasher ColorHasher) {
	hasherMutex.Lock()
	defer hasherMutex.Unlock()

	defaultHasher = hasher
}
