package presentation

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// newDecoder membungkus encoding/xml HANYA untuk memvalidasi bentuk di dalam
// tes. Jalur produksi tidak boleh memakainya untuk menulis: serializer-nya
// menulis ulang prefix namespace dan Word menolak hasilnya.
func newDecoder(data []byte) *xml.Decoder {
	return xml.NewDecoder(bytes.NewReader(data))
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
