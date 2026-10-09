# AGENTS.md

Reglas para agentes de código (y humanos) en este repo. Es **configuración ejecutable**, no onboarding.

## Project

`calculadora-impuestos-arg` — calculadora/agregador de precios digitales en Argentina con impuestos y medios de pago. Parte de la org `mancommunis`.

## Estructura

| Path | Qué |
|---|---|
| `core/` | Motor fiscal en Rust (lógica pura) → compila a WASM |
| `services/` | Go: worker, api, mcp, cli (TUI), sync |
| `web/` | SvelteKit |
| `config/` | config-as-code JSON (fuente de verdad, revisable por PR) |
| `data/` | artefactos generados (Parquet/SQLite) — gitignoreados |

## Comandos

| Tarea | Comando |
|---|---|
| Build motor | `make core` |
| Test motor | `make test-core` |
| Build servicios | `make services` |
| Test servicios | `make test-services` |
| Dev web | `make dev-web` |
| Todo | `make all` |

## Reglas MUST

- **TDD**: test que falla primero, después la implementación (aplica a `core/` y `services/`).
- **Conventional Commits**: `type(scope): subject`. Nunca agregar atribución de IA ni `Co-Authored-By`.
- **El motor no sabe nada de la pantalla**: `core/` es lógica pura — sin I/O, sin red, sin DB.
- **Config-as-code**: cambios de impuestos/comisiones van a `config/*.json`, con `vigente_desde`/`vigente_hasta`.
- **cgo**: solo el `data-worker` puede usar cgo. Todo el resto compila con `CGO_ENABLED=0`.
- **Secretos**: nunca en git. `.env` gitignoreado; `.env.example` con placeholders; keys de proveedores en el keychain del SO.
- **Nunca loguear** keys ni tokens.

## Lenguaje

- **Dominio en español**: vocabulario de negocio (`regimen`, `medio_pago`, `cotizacion`, `provincia`) en identificadores y config.
- **Técnico/infra en inglés**: frameworks, utilidades genéricas, conceptos HTTP.
- Ver ambos en el mismo archivo es lo esperado, no algo para "corregir".

## Docs y proceso

- **Context7** para documentación de librerías.
- Plan de arranque: `~/.opencode/plan/mancommunis-calculadora-impuestos-arg.md`.
- Cambios no triviales: proceso SDD (proposal → spec → tasks → apply → verify).

## No hacer

- No commitear `data/` ni `.env`.
- No introducir cgo fuera del `data-worker`.
- No meter lógica de cálculo fuera de `core/`.
