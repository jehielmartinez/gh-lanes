#!/usr/bin/env bash
# Builds the `gh lanes` extension assets for cli/gh-extension-precompile.
# gh picks the asset whose name ends in <os>-<arch>, so the names must stay in
# that form. The platforms match .goreleaser.yaml rather than the action's
# default list, which includes Windows and FreeBSD.
set -euo pipefail

tag="$1"
mkdir -p dist
for platform in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64; do
  GOOS="${platform%-*}" GOARCH="${platform#*-}" CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w -X main.version=${tag#v}" \
    -o "dist/gh-lanes_${tag}_${platform}" .
done
