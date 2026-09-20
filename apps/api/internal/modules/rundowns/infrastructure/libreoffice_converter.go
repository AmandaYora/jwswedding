package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// LibreOfficeConverter mengubah .docx hasil renderer menjadi PDF dengan
// memanggil `soffice --headless`.
//
// Kenapa mengonversi berkas yang SAMA, bukan menggambar ulang PDF-nya dengan
// fpdf yang sudah ada di repo: dua implementasi desain akan saling menyimpang
// begitu template disunting, dan tuntutan fiturnya justru "PDF harus identik
// dengan DOCX".
//
// Yang tetap berbeda dan diterima sadar: Cambria/Calibri milik Microsoft tidak
// boleh diedarkan di dalam image, jadi container memasang padanan
// metric-compatible (Caladea/Carlito/Liberation). Karena metrik hurufnya sama,
// pemenggalan baris dan posisi halaman identik; yang berbeda hanya bentuk
// huruf secara halus.
type LibreOfficeConverter struct {
	binary  string
	timeout time.Duration
	// sem membatasi konversi menjadi SATU pada satu waktu. VPS produksi hanya
	// ~2 GB RAM dan juga menjalankan MySQL, sementara satu proses soffice
	// memakan ratusan MB — dua klik Generate yang bersamaan bisa membuatnya
	// kehabisan memori.
	sem chan struct{}
}

func NewLibreOfficeConverter(binary string, timeout time.Duration) *LibreOfficeConverter {
	if binary == "" {
		binary = "soffice"
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &LibreOfficeConverter{
		binary:  binary,
		timeout: timeout,
		sem:     make(chan struct{}, 1),
	}
}

// Available melaporkan apakah biner LibreOffice benar-benar ada. Dipakai
// handler untuk menjawab "PDF tidak tersedia di lingkungan ini" secara jelas,
// alih-alih membiarkan pemanggilan gagal dengan pesan exec yang membingungkan
// (dev di Windows umumnya tidak memasang LibreOffice).
func (c *LibreOfficeConverter) Available() bool {
	if filepath.IsAbs(c.binary) {
		_, err := os.Stat(c.binary)
		return err == nil
	}
	_, err := exec.LookPath(c.binary)
	return err == nil
}

func (c *LibreOfficeConverter) ConvertToPDF(ctx context.Context, docx []byte) ([]byte, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	dir, err := os.MkdirTemp("", "rundown-pdf-")
	if err != nil {
		return nil, fmt.Errorf("gagal menyiapkan direktori sementara: %w", err)
	}
	defer os.RemoveAll(dir)

	in := filepath.Join(dir, "rundown.docx")
	if err := os.WriteFile(in, docx, 0o600); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// UserInstallation per-permintaan WAJIB: tanpa itu instance soffice
	// berebut profil pengguna yang sama dan yang kedua langsung gagal.
	profile := filepath.Join(dir, "profile-"+uuid.NewString())
	cmd := exec.CommandContext(ctx, c.binary,
		"--headless", "--norestore", "--nolockcheck", "--nodefault",
		"-env:UserInstallation=file:///"+filepath.ToSlash(profile),
		"--convert-to", "pdf", "--outdir", dir, in)
	cmd.Env = append(os.Environ(), "HOME="+dir)

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("konversi PDF melewati batas %s", c.timeout)
	}
	if err != nil {
		return nil, fmt.Errorf("konversi PDF gagal: %w (%s)", err, truncate(string(out), 300))
	}

	pdf, err := os.ReadFile(filepath.Join(dir, "rundown.pdf"))
	if err != nil {
		return nil, fmt.Errorf("konversi PDF tidak menghasilkan berkas: %w", err)
	}
	if len(pdf) == 0 {
		return nil, fmt.Errorf("konversi PDF menghasilkan berkas kosong")
	}
	return pdf, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
