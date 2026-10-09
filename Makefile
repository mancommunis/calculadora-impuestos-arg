.PHONY: all help core test-core services test-services agente scrape tui web dev-web

all: core services web

help:
	@echo "make core          - build del motor Rust (WASM: wasip1 + unknown)"
	@echo "make test-core     - tests del motor (Rust)"
	@echo "make services      - build de servicios Go (CGO_ENABLED=0)"
	@echo "make test-services - tests de servicios (Go) [depende de core]"
	@echo "make agente        - corre el agente de config en local"
	@echo "make scrape        - actualiza precios del catálogo (Steam; Battle.net bloqueado)"
	@echo "make tui           - abre la TUI (Go + Bubble Tea)"
	@echo "make web           - build de la web (SvelteKit)"
	@echo "make dev-web       - dev server de SvelteKit"

core:
	cd core && cargo build --release --target wasm32-wasip1
	cd core && cargo build --release --target wasm32-unknown-unknown

test-core:
	cd core && cargo test

services:
	cd services && CGO_ENABLED=0 go build ./...

test-services: core
	cd services && CGO_ENABLED=0 go test ./...

agente:
	cd services && go run ./cmd/agente -fuentes ../config/fuentes.json -estado ../agente/estado.json -out ../agente

scrape:
	cd services && go run ./cmd/scraper -dataset ../dataset/catalogo.json

tui:
	cd services && go run ./cmd/tui

web: core
	cd web && npm run build

dev-web: core
	cd web && npm run dev
