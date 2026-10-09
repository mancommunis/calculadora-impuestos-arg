# calculadora-impuestos-arg

> ¿Sos nuevo en el proyecto? Leé **[`docs/ONBOARDING.md`](./docs/ONBOARDING.md)**.

Precio final de compras digitales en Argentina — **juegos, suscripciones de streaming y servicios de IA/tokens** — con impuestos, cotizaciones y comparación de métodos de pago.

No es un formulario: es un **agregador + dato real + local-first + CLI/TUI + MCP + BYOK**.

## Stack

| Capa | Tecnología |
|---|---|
| Motor fiscal | **Rust → WASM** |
| Servicios / worker / CLI / MCP | **Go** (+ `wazero` para cargar el motor WASM) |
| Web | **SvelteKit** (Svelte 5) |
| DB operativa | **SQLite pure-Go** (`modernc.org/sqlite`) |
| DB compartida | **Turso / libSQL** (embedded replicas, cgo) |
| Analítica | **DuckDB** (`go-duckdb`, cgo) |
| IA | **DeepSeek** (BYOK) |

Política de **cuarentena de cgo**: solo el `data-worker` usa cgo; el resto compila con `CGO_ENABLED=0`.

## Estructura

```
core/        Rust: motor fiscal puro → WASM
services/    Go: worker, api, mcp, cli (TUI), sync
web/         SvelteKit
config/      config-as-code (tax, wallets, subscriptions, providers)
data/        artefactos generados (Parquet/SQLite) — no se commitean
```

## Estado

**Fase 0:** scaffold + convenciones. Plan completo en `~/.opencode/plan/mancommunis-calculadora-impuestos-arg.md`.

## Licencia

AGPL-3.0. Ver [`LICENSE`](./LICENSE).
