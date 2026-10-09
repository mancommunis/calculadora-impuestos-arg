.PHONY: all help core test-core services test-services web dev-web

all: core services web

help:
	@echo "make core          - build del motor Rust (WASM: wasip1 + unknown)"
	@echo "make test-core     - tests del motor (Rust)"
	@echo "make services      - build de servicios Go (CGO_ENABLED=0)"
	@echo "make test-services - tests de servicios (Go) [depende de core]"
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

web: core
	cd web && npm run build

dev-web: core
	cd web && npm run dev
