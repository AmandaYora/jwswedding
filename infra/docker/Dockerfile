# Multi-stage build: static frontend + Go API in one container.
FROM node:20-alpine AS web
WORKDIR /repo
COPY package.json package-lock.json ./
COPY apps/web ./apps/web
RUN npm install --workspace=apps/web --include-workspace-root=false
RUN npm run build -w apps/web

FROM golang:1.25-alpine AS api
WORKDIR /src
COPY apps/api ./apps/api
WORKDIR /src/apps/api
# CGO_ENABLED=0 bukan optimasi, melainkan syarat: tahap build ini Alpine (musl)
# sementara runtime di bawah Debian (glibc). Selama runtime-nya masih Alpine
# keduanya selalu cocok, jadi ini tidak pernah perlu ditulis; sejak runtime
# pindah ke Debian, binary yang ter-link dinamis ke musl tidak akan jalan sama
# sekali di sana — dan gagalnya saat container menyala, dengan pesan "not
# found" yang menyesatkan. Seluruh dependensi di go.mod murni Go, jadi
# mematikan cgo tidak menghilangkan apa pun.
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/server

# Runtime is Debian, not Alpine, because the Rundown feature converts its
# .docx to PDF with LibreOffice (docs/plan/rundown-generator/PLAN.md D1) and
# Debian is where the metric-compatible font packages below actually exist.
#
# What this costs, stated plainly: the image grows from ~25 MB to ~500 MB, and
# every deploy pulls that (the VPS never builds — docs/DEPLOYMENT.md §6).
#
# Fonts: Cambria and Calibri are Microsoft's and must not be redistributed in
# an image. Caladea and Carlito are metric-compatible substitutes (identical
# character widths), so line breaks and page positions come out the same as in
# Word; only the letterforms differ subtly. Liberation covers Times New Roman
# and Arial the same way. Without these three packages the PDF repaginates and
# stops matching the .docx, which is the whole point of the feature.
#
# No JRE: LibreOffice needs Java only for Base and some filters, not for a
# Writer-to-PDF conversion. Leaving it out saves a few hundred MB.
FROM debian:bookworm-slim
#
# `libreoffice-writer` dipilih, bukan varian `-nogui` yang lebih ramping:
# yang ini sudah pasti ada di bookworm. Kalau ukuran image jadi masalah,
# `-nogui` layak dicoba — tapi buktikan dulu dengan smoke test di bawah,
# jangan ditukar begitu saja.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libreoffice-writer \
        fonts-crosextra-caladea \
        fonts-crosextra-carlito \
        fonts-liberation2 \
        fontconfig \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*
# Cache fontconfig dibangun sekarang, bukan saat konversi PDF pertama. Tanpa
# ini pemakai pertama setelah container menyala menanggung pembangunan cache
# di ATAS 3-6 detik konversi, dan di VPS 2 GB itu bisa mendekati batas 60
# detik milik konverter — terbaca sebagai "Gagal membuat PDF" padahal hanya
# lambat sekali.
RUN fc-cache -f
RUN groupadd -r app && useradd -r -g app -d /home/app -m app
WORKDIR /app
COPY --from=api /out/api ./api
COPY --from=web /repo/apps/web/dist ./public
RUN chown -R app:app /app
USER app
# soffice writes a user profile on first run and refuses to start without a
# writable HOME. The converter also passes its own -env:UserInstallation per
# request, but HOME still has to exist for the non-root user.
ENV HOME=/home/app
ENV APP_PORT=8080
EXPOSE 8080
ENTRYPOINT ["./api"]
