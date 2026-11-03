#!/usr/bin/env bash
# Builds the Windows app from any OS (no Windows machine or C toolchain
# needed — the Windows build is pure Go):
#
#   packaging/windows/build.sh                 # amd64 (most PCs)
#   ARCH=arm64 packaging/windows/build.sh      # Windows on ARM (Surface, Snapdragon laptops)
#   VERSION=0.3.0 packaging/windows/build.sh
#
# Output in dist/windows-<arch>/:
#   Loomless.exe  — double-click app for QA: native window, no console
#   loomless-cli.exe — the same program as a console CLI (record, export, …)
#
# Both carry the app icon, version info, and a manifest declaring
# per-monitor DPI awareness (so screenshots are crisp and correctly
# positioned on scaled displays) and long-path support.
set -euo pipefail

cd "$(dirname "$0")/../.."
VERSION="${VERSION:-0.0.0-dev}"
ARCH="${ARCH:-amd64}"
OUT="dist/windows-$ARCH"
# Windows version resources need four plain numbers.
NUMERIC="$(echo "$VERSION" | sed -E 's/[^0-9.].*$//; s/\.$//')"
while [[ "$(echo "$NUMERIC" | tr -cd . | wc -c)" -lt 3 ]]; do NUMERIC="$NUMERIC.0"; done

work="$(mktemp -d)"
syso="cmd/loomless/rsrc_windows_$ARCH.syso"
trap 'rm -rf "$work" "$syso"' EXIT

cp assets/icon-512.png "$work/icon.png"
sed -e "s/__VERSION__/$VERSION/g" -e "s/__NUMERIC__/$NUMERIC/g" \
  packaging/windows/winres.json > "$work/winres.json"
go run github.com/tc-hib/go-winres@v0.3.3 make \
  --in "$work/winres.json" --arch "$ARCH" --out "$work/rsrc"
mv "$work/rsrc_windows_$ARCH.syso" "$syso"

rm -rf "$OUT"
mkdir -p "$OUT"
export GOOS=windows GOARCH="$ARCH" CGO_ENABLED=0
go build -trimpath -ldflags "-s -w -H=windowsgui" -o "$OUT/Loomless.exe" ./cmd/loomless
go build -trimpath -ldflags "-s -w" -o "$OUT/loomless-cli.exe" ./cmd/loomless

echo "built $OUT/Loomless.exe and $OUT/loomless-cli.exe ($VERSION)"
