#!/usr/bin/env bash
set -euo pipefail

PLATFORMS=(
  "windows/amd64"
  "linux/amd64"
  "darwin/amd64"
)

for pair in "${PLATFORMS[@]}"; do
  GOOS="${pair%/*}"
  GOARCH="${pair#*/}"
  output="build/echo-rebuild-$GOOS-$GOARCH"
  if [ "$GOOS" = "windows" ]; then
    output="$output.exe"
  fi
  echo "Building $output ..."
  CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" go build -o "$output" ./cmd/echo/
done

echo "All builds complete."
