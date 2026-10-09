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
OUT="$(cd "$OUT" && pwd)"
PACKAGE_STAGE="$(mktemp -d)"
trap 'rm -rf "$PACKAGE_STAGE"' EXIT
mkdir -p "$PACKAGE_STAGE/configs" "$PACKAGE_STAGE/deploy/systemd" "$PACKAGE_STAGE/docs"
cp LICENSE NOTICE README.md "$PACKAGE_STAGE/"
cp configs/tayga.example.yaml "$PACKAGE_STAGE/configs/"
cp deploy/systemd/tayga.service "$PACKAGE_STAGE/deploy/systemd/"
cp docs/setup.md docs/migration.md docs/setup-filters.md docs/release-0.9.4.md "$PACKAGE_STAGE/docs/"
archives=()

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
    -o "${PACKAGE_STAGE}/tayga-mail${ext}" ./cmd/tayga
  if [[ "$goos" == "windows" ]]; then
    (cd "$PACKAGE_STAGE" && zip -qr "${OUT}/${name}.zip" "tayga-mail${ext}" LICENSE NOTICE README.md configs/tayga.example.yaml deploy docs)
    archives+=("${name}.zip")
  else
    COPYFILE_DISABLE=1 tar -czf "${OUT}/${name}.tar.gz" -C "$PACKAGE_STAGE" tayga-mail LICENSE NOTICE README.md configs/tayga.example.yaml deploy docs
    archives+=("${name}.tar.gz")
  fi
  rm -f "${PACKAGE_STAGE}/tayga-mail${ext}"
done

(
  cd "$OUT"
  shasum -a 256 "${archives[@]}" | tee checksums.txt
)

echo "done."
ls -la "$OUT"
