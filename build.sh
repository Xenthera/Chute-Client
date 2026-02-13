#!/usr/bin/env bash
set -e

# architectures and operating systems
OS=("linux" "darwin" "windows")
ARCH=("amd64" "arm64")

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
OUT_DIR="$ROOT_DIR/bin"

# create output folder
mkdir -p "$OUT_DIR"

for os in "${OS[@]}"; do
  for arch in "${ARCH[@]}"; do
    outdir="$OUT_DIR/$os/$arch"
    mkdir -p "$outdir"

    outfile="$outdir/chute"
    [ "$os" = "windows" ] && outfile="$outfile.exe"

    echo "Building $os/$arch -> $outfile"
    (cd "$BACKEND_DIR" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o "$outfile" ./)
  done
done

echo "Builds completed in ./bin/"