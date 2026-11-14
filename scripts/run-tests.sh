#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
export GOWORK=off
# Offline suite: native libavif oracles are a separate, versioned acceptance.
# Point AVIFDEC_BIN at a native binary explicitly to opt into those comparisons.
export AVIFDEC_BIN="${AVIFDEC_BIN:-$PWD/.offline-no-avifdec}"
for GAV1D_TOOLCHAIN in go1.27.3 go1.26.10; do
  echo "Testing private gav1d with $GAV1D_TOOLCHAIN"
  GOTOOLCHAIN="$GAV1D_TOOLCHAIN" go test -race ./...
  GOTOOLCHAIN="$GAV1D_TOOLCHAIN" go test -tags noasm -run 'Test(PictureToNRGBA|DisableScratchPooling|UnpooledTaskCleanup)' ./...
  GOTOOLCHAIN="$GAV1D_TOOLCHAIN" go vet ./...
  CGO_ENABLED=0 GOTOOLCHAIN="$GAV1D_TOOLCHAIN" go build ./...
done
