# Provenance of `gopher.lossless.webp`

Copied verbatim from `golang.org/x/image@v0.44.0`'s own test fixtures
(`testdata/gopher-doc.1bpp.lossless.webp`), used here to exercise
`appicon.Square`'s WebP decode path with a real WebP file.

`golang.org/x/image/webp` only implements a *decoder* — there is no pure-Go
WebP encoder available (libwebp's encoder requires cgo), so this fixture
can't be synthesized in the test itself the way the PNG/JPEG fixtures are
(see `makePNG`/`makeJPEG` in `appicon_test.go`).

- Source: https://cs.opensource.google/go/x/image (module `golang.org/x/image`)
- License: BSD-3-Clause — see `golang.org/x/image`'s own `LICENSE` file,
  reproduced at `$(go env GOMODCACHE)/golang.org/x/image@v0.44.0/LICENSE`.
- Copyright: The Go Authors.
