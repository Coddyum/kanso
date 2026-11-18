#!/bin/bash
# Construit l'app Mac « Coddyum Inc » (une fenêtre sur le serveur local + les rappels)
# et l'installe dans ~/Applications. Le serveur, lui, tourne grâce à `make install-agent`.
set -euo pipefail
cd "$(dirname "$0")/.."
OUT="bin/Coddyum Inc.app"
DEST="$HOME/Applications/Coddyum Inc.app"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

rm -rf "$OUT"
mkdir -p "$OUT/Contents/MacOS" "$OUT/Contents/Resources"
swiftc -O -swift-version 5 tools/app/main.swift -o "$OUT/Contents/MacOS/CoddyumInc"
cp tools/app/Info.plist "$OUT/Contents/Info.plist"

swiftc -O -swift-version 5 tools/app/icon.swift -o "$TMP/icon"
"$TMP/icon" "$TMP/icon.png"
mkdir "$TMP/AppIcon.iconset"
for s in 16 32 128 256 512; do
    sips -z "$s" "$s" "$TMP/icon.png" --out "$TMP/AppIcon.iconset/icon_${s}x${s}.png" >/dev/null
    sips -z "$((s * 2))" "$((s * 2))" "$TMP/icon.png" --out "$TMP/AppIcon.iconset/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$TMP/AppIcon.iconset" -o "$OUT/Contents/Resources/AppIcon.icns"
codesign --force --deep -s - "$OUT"

pkill -x CoddyumInc 2>/dev/null && sleep 1 || true
mkdir -p "$HOME/Applications"
rm -rf "$DEST"
cp -R "$OUT" "$DEST"
echo "App installée : $DEST"
