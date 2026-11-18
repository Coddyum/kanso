PORT ?= 48219
# The app's name: scripts/brand.sh (kept equal to internal/brand by its test).
ID := $(shell . scripts/brand.sh && echo $$ID)

.PHONY: build run open logs test install-agent uninstall-agent app

# Avec le lancement automatique installé, l'app tourne déjà : `make build` suffit,
# le lanceur la redémarre sur la nouvelle version en 2 à 3 secondes.
build: bin/pdftext bin/$(ID)-launcher
	go build -trimpath -ldflags "-s -w" -o bin/$(ID) ./cmd/kanso

bin/pdftext: macos/pdftext.swift
	mkdir -p bin
	swiftc -O macos/pdftext.swift -o bin/pdftext

# Le lanceur change rarement : il n'est recompilé que si son code change.
bin/$(ID)-launcher: cmd/kanso-launcher/main.go
	go build -trimpath -ldflags "-s -w" -o bin/$(ID)-launcher ./cmd/kanso-launcher

# À la main, dans un terminal (sans le lancement automatique).
run: build
	./bin/$(ID) -port $(PORT)

open:
	open http://localhost:$(PORT)

logs:
	tail -n 40 "$(HOME)/Library/Logs/$(ID).log"

# Vérifications avant un commit : format, analyse, tests (avec le détecteur de courses).
test:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "↑ fichiers à formater : gofmt -w ." && exit 1)
	go vet ./...
	go test -race ./...

# Lancement automatique (LaunchAgent macOS) : démarre à l'ouverture de session,
# redémarre après un arrêt et après chaque `make build`.
install-agent: build
	./scripts/agent.sh install "$(CURDIR)" $(PORT)

uninstall-agent:
	./scripts/agent.sh uninstall

# L'app Mac (fenêtre + rappels), installée dans ~/Applications. À refaire seulement
# quand macos/app change : l'écran vient du serveur, `make build` suffit pour le reste.
app:
	./scripts/app.sh
