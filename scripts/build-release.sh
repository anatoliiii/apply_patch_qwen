#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"

VERSION="${1:-dry-run}"
CGO_ENABLED="${CGO_ENABLED:-0}"
export CGO_ENABLED

TARGETS=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
)

INCLUDE_FILES=(README.md README.ru.md LICENSE install.sh)

echo "=== build-release.sh ==="
echo "Version: $VERSION"
echo "Root:    $ROOT_DIR"
echo "Dist:    $DIST_DIR"
echo

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

for target in "${TARGETS[@]}"; do
  read -r goos goarch <<<"$target"
  name="apply_patch_qwen_${VERSION}_${goos}_${goarch}"
  stage="$DIST_DIR/$name"
  archive="$DIST_DIR/${name}.tar.gz"

  echo "Building ${goos}/${goarch}..."
  mkdir -p "$stage/bin"

  GOOS="$goos" GOARCH="$goarch" go build \
    -trimpath -ldflags="-s -w" \
    -o "$stage/bin/qwen-apply-patch-mcp" \
    "$ROOT_DIR/cmd/qwen-apply-patch-mcp"

  GOOS="$goos" GOARCH="$goarch" go build \
    -trimpath -ldflags="-s -w" \
    -o "$stage/bin/qwen-apply-patch-tool" \
    "$ROOT_DIR/cmd/qwen-apply-patch-tool"

  for f in "${INCLUDE_FILES[@]}"; do
    if [[ -f "$ROOT_DIR/$f" ]]; then
      cp "$ROOT_DIR/$f" "$stage/"
    fi
  done
  chmod +x "$stage/install.sh"

  tar -C "$DIST_DIR" -czf "$archive" "$(basename "$stage")"
  rm -rf "$stage"
  echo "  -> $archive"
done

echo
echo "Generating checksums..."
(cd "$DIST_DIR" && sha256sum ./*.tar.gz > SHA256SUMS.txt)

echo
echo "=== Release artifacts ==="
ls -lh "$DIST_DIR/"
echo
echo "Done."
