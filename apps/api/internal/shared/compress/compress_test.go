package compress

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func jpegOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func TestToPNG_ConvertsAndDownscales(t *testing.T) {
	out, err := ToPNG(jpegOf(t, 3000, 1500))
	if err != nil {
		t.Fatalf("ToPNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("hasil bukan PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != MaxDimension || b.Dy() != 1000 {
		t.Errorf("ukuran = %dx%d, mau %dx1000", b.Dx(), b.Dy(), MaxDimension)
	}
}

func TestToPNG_KeepsSmallImageSize(t *testing.T) {
	out, err := ToPNG(jpegOf(t, 40, 30))
	if err != nil {
		t.Fatalf("ToPNG: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != 40 || cfg.Height != 30 {
		t.Errorf("ukuran = %dx%d (err %v), mau 40x30", cfg.Width, cfg.Height, err)
	}
}

func TestToPNG_RejectsNonJPEG(t *testing.T) {
	if _, err := ToPNG([]byte("bukan gambar")); err == nil {
		t.Error("masukan bukan JPEG harus menghasilkan galat")
	}
}
