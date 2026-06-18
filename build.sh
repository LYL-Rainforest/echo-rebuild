#!/bin/bash
set -e

mkdir -p build

platforms=("windows/amd64" "linux/amd64" "darwin/amd64")

for p in "${platforms[@]}"; do
    IFS="/" read -r GOOS GOARCH <<< "$p"
    echo "Building $GOOS/$GOARCH..."
    CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH go build -o "build/echo-rebuild-${GOOS}-${GOARCH}" ./cmd/echo/
done

echo "All builds complete."
