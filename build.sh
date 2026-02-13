#!/usr/bin/env bash
set -e

# architectures and operating systems
OS=("linux" "darwin" "windows")
ARCH=("amd64" "arm64")

# create output folder
mkdir -p bin

for os in "${OS[@]}"; do
  for arch in "${ARCH[@]}"; do
    outdir="bin/$os/$arch"
    mkdir -p "$outdir"

    outfile="$outdir/chute"
    [ "$os" = "windows" ] && outfile="$outfile.exe"

    echo "Building $os/$arch -> $outfile"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o "$outfile" ./
  done
done

echo "Builds completed in ./bin/"