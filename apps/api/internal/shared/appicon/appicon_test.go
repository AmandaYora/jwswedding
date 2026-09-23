package appicon

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

// makePNG membuat PNG sungguhan berukuran w×h berwarna solid c — Square
// benar-benar men-decode dan meng-encode ulang, jadi byte sembarangan akan
// gagal begitu saja tanpa membuktikan apa pun soal skala/pemusatan/latar
// (idiom yang sama dengan staff/application's validPNG).
func makePNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png sumber: %v", err)
	}
	return buf.Bytes()
}

// makeJPEG mirrors makePNG, JPEG-encoded instead.
func makeJPEG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg sumber: %v", err)
	}
	return buf.Bytes()
}

// webpFixture membaca sebuah WebP sungguhan dari testdata. golang.org/x/image
// hanya berisi DECODER webp, tidak ada encoder murni-Go (libwebp perlu cgo),
// jadi tidak bisa disintesis di dalam tes seperti makePNG/makeJPEG — fixture
// ini disalin apa adanya dari testdata resmi golang.org/x/image (BSD-3-Clause,
// lihat testdata/README.golang-x-image) khusus supaya jalur decode WebP
// benar-benar teruji, bukan sekadar diasumsikan compile karena importnya ada.
func webpFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/gopher.lossless.webp")
	if err != nil {
		t.Fatalf("baca fixture webp: %v", err)
	}
	return data
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("hasil Square bukan PNG valid: %v", err)
	}
	return img
}

func TestSquare_PNGSumberMenghasilkanPNGPersegiUkuranTepat(t *testing.T) {
	src := makePNG(t, 200, 200, color.RGBA{R: 200, G: 30, B: 30, A: 255})
	out, err := Square(src, 192)
	if err != nil {
		t.Fatalf("Square: %v", err)
	}
	img := decodePNG(t, out)
	b := img.Bounds()
	if b.Dx() != 192 || b.Dy() != 192 {
		t.Fatalf("ukuran = %dx%d, mau 192x192", b.Dx(), b.Dy())
	}
}

func TestSquare_JPEGSumberMenghasilkanPNGPersegiUkuranTepat(t *testing.T) {
	src := makeJPEG(t, 300, 150, color.RGBA{R: 10, G: 120, B: 200, A: 255})
	out, err := Square(src, 512)
	if err != nil {
		t.Fatalf("Square: %v", err)
	}
	img := decodePNG(t, out)
	b := img.Bounds()
	if b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("ukuran = %dx%d, mau 512x512", b.Dx(), b.Dy())
	}
}

// Membuktikan decoder WebP benar-benar terdaftar dan terpakai (§4.3 PLAN) —
// allowedLogoMimeTypes menerima logo WebP, dan tanpa import samar
// golang.org/x/image/webp, image.Decode akan gagal untuk berkas ini.
func TestSquare_WebPSumberMenghasilkanPNGPersegiUkuranTepat(t *testing.T) {
	src := webpFixture(t)
	out, err := Square(src, 180)
	if err != nil {
		t.Fatalf("Square gagal untuk sumber WebP: %v", err)
	}
	img := decodePNG(t, out)
	b := img.Bounds()
	if b.Dx() != 180 || b.Dy() != 180 {
		t.Fatalf("ukuran = %dx%d, mau 180x180", b.Dx(), b.Dy())
	}
}

// Logo lebar (400x100, rasio 4:1) tidak boleh digepengkan jadi kotak — rasio
// asli harus terjaga di dalam kanvas, dan tidak melebihi fitFraction (60%)
// dari sisi kanvas (K4/A4).
func TestSquare_LogoLebarTidakTerdistorsiDanMuatDalamFitFraction(t *testing.T) {
	const size = 200
	src := makePNG(t, 400, 100, color.RGBA{R: 0, G: 150, B: 0, A: 255})
	out, err := Square(src, size)
	if err != nil {
		t.Fatalf("Square: %v", err)
	}
	img := decodePNG(t, out)

	// Cari kotak-pembatas piksel non-putih (logonya) di dalam kanvas.
	minX, minY, maxX, maxY := size, size, -1, -1
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 == 255 && g>>8 == 255 && b>>8 == 255 {
				continue // latar putih
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < 0 {
		t.Fatal("tidak ditemukan piksel logo (non-putih) sama sekali di kanvas")
	}
	logoW := maxX - minX + 1
	logoH := maxY - minY + 1

	wantMaxSide := int(float64(size) * fitFraction)
	if logoW > wantMaxSide+1 { // +1 toleransi pembulatan
		t.Errorf("lebar logo %d melebihi batas fitFraction %d", logoW, wantMaxSide)
	}
	if logoH > wantMaxSide+1 {
		t.Errorf("tinggi logo %d melebihi batas fitFraction %d", logoH, wantMaxSide)
	}

	// Rasio 4:1 sumber harus terjaga (bukan digepengkan jadi persegi) —
	// toleransi kecil untuk pembulatan piksel.
	gotRatio := float64(logoW) / float64(logoH)
	wantRatio := 400.0 / 100.0
	if diff := gotRatio - wantRatio; diff > 0.15 || diff < -0.15 {
		t.Errorf("rasio logo hasil = %.2f, mau sekitar %.2f (sumber 400x100)", gotRatio, wantRatio)
	}
}

// Logo lebih kecil dari kotak target harus DIPERBESAR, bukan dibiarkan
// sekecil sumbernya — slot ikon berukuran tetap, jadi logo kecil yang
// dibiarkan apa adanya akan tampak tenggelam di tengah margin.
func TestSquare_LogoLebihKecilDariTargetDiperbesar(t *testing.T) {
	const size = 512
	src := makePNG(t, 10, 10, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	out, err := Square(src, size)
	if err != nil {
		t.Fatalf("Square: %v", err)
	}
	img := decodePNG(t, out)

	minX, maxX := size, -1
	for x := 0; x < size; x++ {
		r, g, b, _ := img.At(x, size/2).RGBA()
		if r>>8 == 255 && g>>8 == 0 && b>>8 == 0 {
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
		}
	}
	if maxX < 0 {
		t.Fatal("tidak ditemukan piksel merah logo di baris tengah")
	}
	logoW := maxX - minX + 1
	// Sumbernya 10px; hasil harus jauh lebih besar dari 10px (diperbesar
	// mendekati fitFraction*size = ~307px), bukan dibiarkan mendekati 10px.
	if logoW < 100 {
		t.Errorf("logo kecil (10px) tidak diperbesar, lebar hasil hanya %dpx", logoW)
	}
}

// Latar belakang harus putih solid (K5) — periksa keempat pojok kanvas, yang
// pasti berada di luar area logo (logo dipusatkan dan dibatasi fitFraction).
func TestSquare_LatarBelakangPutihSolid(t *testing.T) {
	const size = 300
	src := makePNG(t, 100, 100, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	out, err := Square(src, size)
	if err != nil {
		t.Fatalf("Square: %v", err)
	}
	img := decodePNG(t, out)

	corners := []image.Point{{0, 0}, {size - 1, 0}, {0, size - 1}, {size - 1, size - 1}}
	for _, p := range corners {
		r, g, b, a := img.At(p.X, p.Y).RGBA()
		if r>>8 != 255 || g>>8 != 255 || b>>8 != 255 || a>>8 != 255 {
			t.Errorf("pojok %v = rgba(%d,%d,%d,%d), mau putih solid (255,255,255,255)", p, r>>8, g>>8, b>>8, a>>8)
		}
	}
}

func TestSquare_SumberRusakMengembalikanGalatBukanPanic(t *testing.T) {
	_, err := Square([]byte("bukan gambar sama sekali"), 192)
	if err == nil {
		t.Fatal("mau galat untuk sumber yang tidak bisa di-decode, dapat nil")
	}
}

func TestSquare_SumberKosongMengembalikanGalat(t *testing.T) {
	_, err := Square(nil, 192)
	if err == nil {
		t.Fatal("mau galat untuk sumber kosong, dapat nil")
	}
}

func TestSquare_SizeNolAtauNegatifMengembalikanGalat(t *testing.T) {
	src := makePNG(t, 50, 50, color.White)
	for _, size := range []int{0, -1, -192} {
		if _, err := Square(src, size); err == nil {
			t.Errorf("size=%d: mau galat, dapat nil", size)
		}
	}
}
