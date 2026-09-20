#!/usr/bin/env bash
# Smoke test PLAN rundown-generator T16.
#
# Membuktikan tiga hal di dalam image yang benar-benar akan dideploy:
#   1. biner soffice ada dan bisa jalan headless sebagai user non-root;
#   2. tiga paket font metric-compatible benar-benar terpasang -- kalau tidak,
#      PDF akan mengalir ulang dan berhenti sama dengan .docx-nya;
#   3. konversi .docx -> .pdf menghasilkan jumlah halaman yang sama dengan
#      yang dicetak Word (13 untuk berkas contoh bawaan).
#
# Tidak bisa dijalankan tanpa Docker. Jalankan setelah `docker build`:
#   ./tools/rundown-template/smoke-pdf.sh jwswedding:local
set -euo pipefail

IMAGE="${1:-jwswedding:local}"
EXPECTED_PAGES="${2:-13}"

echo "== 1. soffice tersedia =="
docker run --rm "$IMAGE" sh -lc 'command -v soffice && soffice --version'

echo
echo "== 2. font metric-compatible terpasang =="
docker run --rm "$IMAGE" sh -lc '
  missing=0
  for f in Caladea Carlito "Liberation Serif" "Liberation Sans"; do
    if fc-list | grep -qi "$f"; then
      echo "  ada     : $f"
    else
      echo "  HILANG  : $f"
      missing=1
    fi
  done
  exit $missing
'

echo
echo "== 3. konversi .docx -> .pdf =="
docker run --rm -v "$(pwd)/tools/rundown-template:/in:ro" "$IMAGE" sh -lc "
  set -e
  cp /in/sample-for-smoke.docx /tmp/s.docx
  soffice --headless --norestore -env:UserInstallation=file:///tmp/p \
          --convert-to pdf --outdir /tmp /tmp/s.docx >/dev/null
  pages=\$(grep -ac '/Type[[:space:]]*/Page[^s]' /tmp/s.pdf || true)
  echo \"  halaman PDF: \$pages (mau $EXPECTED_PAGES)\"
  [ \"\$pages\" = \"$EXPECTED_PAGES\" ]
"

echo
echo "SMOKE TEST LULUS"
