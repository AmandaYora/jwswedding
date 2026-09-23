// Package appicon renders a tenant's logo into fixed-size square PNG icons
// for "Add to Home Screen" (PWA manifest icons + apple-touch-icon) — see
// docs/plan/ikon-homescreen-pwa/PLAN.md.
//
// A tenant logo is stored exactly as uploaded, any aspect ratio —
// compress.Image only downscales an oversized image, it never crops or pads
// (see compress.go's downscale) — so handing a wide/tall logo straight to an
// icon slot lets iOS squash it into a square. Square instead decodes the
// logo, scales it down (or up) to fit within a fraction of a square canvas,
// centers it on a solid background, and always re-encodes as PNG regardless
// of the source format — the one format apple-touch-icon reliably supports
// across iOS versions, and the only way to also normalize a WebP logo (which
// compress.Image never decodes at all; the WebP decoder here is what closes
// that gap).
package appicon

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // decoder registration only, via image.Decode's format registry
	"image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // decoder registration only — allowedLogoMimeTypes accepts WebP
)

// fitFraction is how much of the square canvas the logo may occupy — the
// remaining margin is the safe zone Android's maskable-icon spec expects (a
// circular safe zone of 80% diameter centered on the icon), so one rendered
// icon can serve both a plain and a maskable purpose without a second
// variant (PLAN §3.2 K4/A4).
const fitFraction = 0.6

// backgroundColor is the canvas fill behind the logo — solid white (PLAN
// §3.2 K5), the safest default: most logos are designed against a light
// background, and Android's maskable icons need a solid (non-transparent)
// backdrop regardless.
var backgroundColor = color.White

// Square decodes src (PNG, JPEG, or WebP) and renders it centered on a
// size×size white canvas, scaled to fit within fitFraction of the canvas
// while preserving its aspect ratio, then returns the result PNG-encoded.
//
// A logo smaller than the target box is enlarged, never left at native
// size — the icon slot is a fixed size, so shrinking to match a small source
// would just make the icon look tiny inside its own margin.
//
// Returns an error (never a panic) when src can't be decoded as an image or
// size isn't positive — a caller that pipes this straight into an HTTP
// response should surface that as a real error, not a blank icon that
// quietly hides the corruption.
func Square(src []byte, size int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("appicon: size harus positif, got %d", size)
	}

	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("appicon: gagal men-decode gambar sumber: %w", err)
	}
	srcBounds := img.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("appicon: gambar sumber tidak memiliki dimensi yang valid")
	}

	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(backgroundColor), image.Point{}, draw.Src)

	dstW, dstH := fitDimensions(srcW, srcH, float64(size)*fitFraction)
	offsetX := (size - dstW) / 2
	offsetY := (size - dstH) / 2
	dstRect := image.Rect(offsetX, offsetY, offsetX+dstW, offsetY+dstH)
	// draw.Over (not draw.Src, unlike the background fill above): a logo with
	// a semi-transparent/antialiased edge should blend into the white
	// backdrop rather than punch a hard-edged hole in it.
	draw.CatmullRom.Scale(canvas, dstRect, img, srcBounds, draw.Over, nil)

	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, fmt.Errorf("appicon: gagal meng-encode PNG: %w", err)
	}
	return out.Bytes(), nil
}

// fitDimensions scales (srcW, srcH) so its longest side equals maxSide,
// preserving aspect ratio exactly — same idiom as compress.go's downscale,
// generalized to also enlarge (downscale only ever shrinks). Always returns
// at least 1×1, so a degenerate maxSide never yields an empty destination
// rectangle for draw.CatmullRom.Scale.
func fitDimensions(srcW, srcH int, maxSide float64) (dstW, dstH int) {
	if srcW >= srcH {
		dstW = int(maxSide)
		dstH = int(maxSide * float64(srcH) / float64(srcW))
	} else {
		dstH = int(maxSide)
		dstW = int(maxSide * float64(srcW) / float64(srcH))
	}
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}
	return dstW, dstH
}
