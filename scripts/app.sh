#!/bin/bash
# Construit l'app Mac (une fenêtre sur le serveur local + les rappels) et
# l'installe dans ~/Applications. Le serveur, lui, tourne grâce à `make install-agent`.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/brand.sh
OUT="bin/$NAME.app"
DEST="$HOME/Applications/$NAME.app"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

rm -rf "$OUT"
mkdir -p "$OUT/Contents/MacOS" "$OUT/Contents/Resources"
swiftc -O -swift-version 5 macos/app/main.swift -o "$OUT/Contents/MacOS/$NAME"
# The sentence macOS shows when it asks for the Downloads folder: English by
# default, each language of web/locales in its own InfoPlist.strings.
usage() { plutil -extract mac.downloadsUsage raw -o - "$1"; }
sed -e "s|@NAME@|$NAME|g" -e "s|@BUNDLE_ID@|$BUNDLE_ID|g" -e "s|@DOWNLOADS@|$(usage web/locales/en.json | sed 's/[|&]/\\&/g')|" \
    macos/app/Info.plist > "$OUT/Contents/Info.plist"
for f in web/locales/*.json; do
    lang=$(basename "$f" .json)
    [ "$lang" = zh ] && lang=zh-Hans
    mkdir -p "$OUT/Contents/Resources/$lang.lproj"
    text=$(usage "$f")
    printf '"NSDownloadsFolderUsageDescription" = "%s";\n' "${text//\"/\\\"}" > "$OUT/Contents/Resources/$lang.lproj/InfoPlist.strings"
done

swiftc -O -swift-version 5 macos/app/icon.swift -o "$TMP/icon"
"$TMP/icon" "$TMP/icon.png"
mkdir "$TMP/AppIcon.iconset"
for s in 16 32 128 256 512; do
    sips -z "$s" "$s" "$TMP/icon.png" --out "$TMP/AppIcon.iconset/icon_${s}x${s}.png" >/dev/null
    sips -z "$((s * 2))" "$((s * 2))" "$TMP/icon.png" --out "$TMP/AppIcon.iconset/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$TMP/AppIcon.iconset" -o "$OUT/Contents/Resources/AppIcon.icns"
codesign --force --deep -s - "$OUT"

pkill -x "$NAME" 2>/dev/null && sleep 1 || true
mkdir -p "$HOME/Applications"
rm -rf "$DEST"
cp -R "$OUT" "$DEST"
echo "App installée : $DEST"
