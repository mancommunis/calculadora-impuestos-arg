# services (Go)

Servicios de orquestación:

- `worker` / `data-worker` — scraping + DuckDB + Turso embedded replicas (**único con cgo**).
- `api` — API mínima.
- `mcp` — servidor MCP (`mcp-go`).
- `cli` — TUI (Bubble Tea).

Todo el resto compila con `CGO_ENABLED=0`. **Fase 2.**
