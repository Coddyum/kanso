#!/bin/bash
# Installe ou retire le LaunchAgent qui lance coddyum-inc à l'ouverture de session,
# par son lanceur : il la relance après un arrêt et après chaque nouvelle version.
set -euo pipefail
LABEL="me.coddyum.inc"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"

case "${1:-}" in
install)
    DIR="$2"; PORT="$3"
    mkdir -p "$HOME/Library/LaunchAgents" "$HOME/Library/Logs"
    cat > "$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>$LABEL</string>
    <key>ProgramArguments</key>
    <array>
        <string>$DIR/bin/coddyum-inc-launcher</string>
        <string>$DIR/bin/coddyum-inc</string>
        <string>-port</string><string>$PORT</string>
    </array>
    <key>WorkingDirectory</key><string>$DIR</string>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>ProcessType</key><string>Background</string>
    <key>LowPriorityIO</key><true/>
    <key>StandardOutPath</key><string>$HOME/Library/Logs/coddyum-inc.log</string>
    <key>StandardErrorPath</key><string>$HOME/Library/Logs/coddyum-inc.log</string>
</dict>
</plist>
PLIST
    launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
    launchctl bootstrap "gui/$(id -u)" "$PLIST"
    echo "coddyum-inc tourne : http://localhost:$PORT"
    ;;
uninstall)
    launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
    rm -f "$PLIST"
    echo "LaunchAgent retiré"
    ;;
*)
    echo "usage : $0 install <dossier> <port> | uninstall" >&2
    exit 1
    ;;
esac
