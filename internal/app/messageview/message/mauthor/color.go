package mauthor

import (
	"fmt"
	"hash"
	"image/color"
	"math"
	"sync"
)

// ColorHasher describes a string hasher that outputs a color.
type ColorHasher interface {
	Hash(name string) color.RGBA
}

var (
	// FNVHasher  = func() hash.Hash32 { return fnv.New32a() }

	// DJB2Hasher is the string hasher used for color hashing.
	DJB2Hasher = newDJB32

	// LightColorHasher generates a pastel color name for use with a dark
	// background.
	LightColorHasher ColorHasher = HSVHasher{
		DJB2Hasher,
		[2]float64{0.3, 0.4},
		[2]float64{0.9, 1.0},
	}

	// DarkColorHasher generates a darker, stronger color name for use with a
	// light background.
	DarkColorHasher ColorHasher = HSVHasher{
		DJB2Hasher,
		[2]float64{0.9, 1.0},
		[2]float64{0.6, 0.7},
	}
)

// RGBHex converts the given color to a HTML hex color string. The alpha value
// is ignored.
func RGBHex(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// HSVHasher describes a color hasher that accepts saturation and value
// parameters in the HSV color space.
type HSVHasher struct {
	H func() hash.Hash32 // hashing function
	S [2]float64         // saturation
	V [2]float64         // value
}

const (
	nHue = 32 // hue count
	nSat = 10
	nVal = 10
)

// Hash hashes the given name using the parameters inside HSVHasher.
func (h HSVHasher) Hash(name string) color.RGBA {
	hasher := h.H()
	hasher.Write([]byte(name))

	// Hash will be within [0, 1].
	hash := float64(hasher.Sum32()) / math.MaxUint32

	hue := hashClamp(hash, 0, 360, nHue)
	sat := hashClamp(hash, h.S[0], h.S[1], nSat)
	val := hashClamp(hash, h.V[0], h.V[1], nVal)

	return hsvrgb(hue, sat, val)
}

// hashClamp converts the given u32 hash to a number within [min, max],
// optionally rounded if round is not 0. Hash must be within [0, 1].
func hashClamp(hash, min, max, round float64) float64 {
	if round > 0 {
		hash = math.Round(hash*round) / round
	}

	r := max - min
	n := min + (hash * r)

	return n
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
