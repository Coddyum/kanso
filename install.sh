#!/bin/bash
# Installs Kanso on this Mac from the clone: checks macOS, Apple's command
# line tools and Go, builds, installs the Mac app, then runs `kanso init`
# (the folder, the start at login). This script downloads nothing: when
# something is missing, it says where to get it, then stops.
set -euo pipefail
cd "$(dirname "$0")"
source scripts/brand.sh

# The user's language, to read the sentences of web/locales (plutil comes with macOS).
lang=""
for v in "${LC_ALL:-}" "${LANG:-}"; do
    code="${v:0:2}"
    [ -f "web/locales/$code.json" ] && lang="$code" && break
done
if [ -z "$lang" ]; then
    first="$(defaults read -g AppleLanguages 2>/dev/null | tr -d ' "()\n' | cut -d, -f1)"
    code="${first:0:2}"
    [ -f "web/locales/$code.json" ] && lang="$code" || lang="en"
fi
text() { plutil -extract "cli.install.$1" raw -o - "web/locales/$lang.json"; }
step() { printf '\n%s\n' "$(text "$1")"; }
stop() { printf '\n✗ %s\n' "$(text "$1")" >&2; exit 1; }

printf '%s\n' "$(plutil -extract cli.init.hello raw -o - "web/locales/$lang.json")"

macos="$(sw_vers -productVersion)"
[ "${macos%%.*}" -ge 13 ] || stop macos

if ! xcode-select -p >/dev/null 2>&1; then
    xcode-select --install >/dev/null 2>&1 || true
    stop xcode
fi

need="$(sed -n 's/^go \([0-9.]*\).*/\1/p' go.mod)"
if ! command -v go >/dev/null 2>&1; then
    open "https://go.dev/dl/" || true
    stop go
fi
have="$(GOTOOLCHAIN=local go env GOVERSION | sed 's/^go//')"
if [ "$(printf '%s\n%s\n' "$need" "$have" | sort -V | head -n1)" != "$need" ]; then
    open "https://go.dev/dl/" || true
    stop goOld
fi

# Contributors: the check that keeps personal files and data out of commits.
[ -d .git ] && git config core.hooksPath .githooks

step building
make build
step app
make app
./bin/"$ID" init "$@"
