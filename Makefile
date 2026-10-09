.PHONY: all help core test-core services test-services dev-web

all: core services dev-web

help:
	@echo "make core          - build del motor Rust (WASM)"
	@echo "make test-core     - tests del motor"
	@echo "make services      - build de servicios Go"
	@echo "make test-services - tests de servicios"
	@echo "make dev-web       - dev server de SvelteKit"

core:
	@echo "[core] pendiente (Fase 1)"

test-core:
	@echo "[core] pendiente (Fase 1)"

services:
	@echo "[services] pendiente (Fase 2)"

test-services:
	@echo "[services] pendiente (Fase 2)"

dev-web:
	@echo "[web] pendiente (Fase 3)"
