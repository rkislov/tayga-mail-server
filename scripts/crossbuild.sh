#!/usr/bin/env bash
# Cross-compile tayga-mail (CGO_ENABLED=0) for common platforms.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
OUT="${OUT:-dist}"

LDFLAGS="-s -w -X github.com/tayga/tms/internal/version.Version=${VERSION} -X github.com/tayga/tms/internal/version.Commit=${COMMIT} -X github.com/tayga/tms/internal/version.Date=${DATE}"

mkdir -p "$OUT"

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
  "freebsd amd64"
)

echo "building tayga-mail ${VERSION} (${COMMIT}) → ${OUT}/"
for t in "${targets[@]}"; do
  set -- $t
  goos=$1
  goarch=$2
  name="tayga-mail_${VERSION}_${goos}_${goarch}"
  ext=""
  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
  fi
  echo "  ${goos}/${goarch}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags="$LDFLAGS" \
    -o "${OUT}/${name}${ext}" ./cmd/tayga
  (
    cd "$OUT"
    if [[ "$goos" == "windows" ]]; then
      zip -q "${name}.zip" "${name}${ext}"
      rm -f "${name}${ext}"
    else
      tar -czf "${name}.tar.gz" "${name}${ext}"
      rm -f "${name}${ext}"
    fi
  )
done

(
  cd "$OUT"
  shasum -a 256 *.tar.gz *.zip 2>/dev/null | tee checksums.txt
)

echo "done."
ls -la "$OUT"
