#!/usr/bin/env bash
# Builds dist/Loomless.app — a double-clickable macOS app that opens the
# dashboard in its own window (see internal/desktop).
#
#   packaging/macos/build-app.sh            # this Mac's architecture
#   UNIVERSAL=1 packaging/macos/build-app.sh  # Apple Silicon + Intel in one app
#   VERSION=0.3.0 packaging/macos/build-app.sh
#
# The bundle is ad-hoc signed, which is enough to run on the Mac that
# built it. Handing it to other people without a Gatekeeper warning
# needs a Developer ID signature and notarization — see PUBLISHING.md.
#
# Because the app's own binary also runs the background click daemon,
# macOS asks for Accessibility and Screen Recording permission for
# "Loomless" itself, not for Terminal.
set -euo pipefail

cd "$(dirname "$0")/../.."
VERSION="${VERSION:-0.0.0-dev}"
APP="dist/Loomless.app"
CONTENTS="$APP/Contents"

rm -rf "$APP"
mkdir -p "$CONTENTS/MacOS" "$CONTENTS/Resources"

LDFLAGS="-s -w"
if [[ "${UNIVERSAL:-}" == "1" ]]; then
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  for arch in arm64 amd64; do
    echo "building $arch…"
    CGO_ENABLED=1 GOARCH="$arch" go build -ldflags "$LDFLAGS" -o "$tmp/loomless-$arch" ./cmd/loomless
  done
  lipo -create -output "$CONTENTS/MacOS/loomless" "$tmp/loomless-arm64" "$tmp/loomless-amd64"
else
  echo "building $(go env GOARCH)…"
  CGO_ENABLED=1 go build -ldflags "$LDFLAGS" -o "$CONTENTS/MacOS/loomless" ./cmd/loomless
fi

sed "s/__VERSION__/$VERSION/g" packaging/macos/Info.plist > "$CONTENTS/Info.plist"

# App icon: scale the 512px master into an .iconset and pack it. (No
# 1024px master exists yet, so the 512@2x slot is left out; Finder
# falls back to the 512 cleanly.)
iconset="$(mktemp -d)/Loomless.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" assets/icon-512.png --out "$iconset/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  if (( double <= 512 )); then
    sips -z "$double" "$double" assets/icon-512.png --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
  fi
done
iconutil -c icns "$iconset" -o "$CONTENTS/Resources/Loomless.icns"
rm -rf "$(dirname "$iconset")"

codesign --force --sign - "$APP"

echo "built $APP ($VERSION)"
