# Onboarding — calculadora-impuestos-arg

Guía para levantar y entender el proyecto. Si sos nuevo (hola, dev-leo 👋), empezá por acá.

## Qué es

Herramienta **open source y gratuita** que calcula el **precio final de compras digitales en Argentina** — juegos, suscripciones de streaming y servicios de IA — con **impuestos, cotizaciones del día y comparación de medios de pago** (todos los bancos y billeteras).

**No es un formulario** (como Impuestito): es un **agregador + datos reales + local-first + CLI/TUI + MCP + BYOK**.

- **Org:** `mancommunis` (organización de software libre). Una herramienta = un repo.
- **Proyecto:** `calculadora-impuestos-arg`.
- **Licencia:** AGPL-3.0 (ver `LICENSE`).

## Arquitectura: un motor, N hosts

El corazón es un **motor fiscal en Rust** que compila a **WASM**. El **mismo archivo `.wasm`** corre en:

- **Go** (servicios, CLI/TUI, MCP) vía [`wazero`](https://wazero.io) — sin CGO.
- **Navegador** (web) vía la WebAssembly API — sin glue.

O sea: **una sola implementación de la ley fiscal, en todos lados.** El motor es lógica pura: sin I/O, sin red, sin DB ("no sabe nada de la pantalla").

```
core/        Rust: motor fiscal puro → WASM (wasm32-wasip1 para Go, wasm32-unknown-unknown para el browser)
services/    Go: api, tui, scraper, agente, llave + internal/{configuracion,motor,tiendas,agente,llaves}
web/         SvelteKit 3 (Svelte 5)
config/      config-as-code: tax-config, wallets, subscriptions, providers, fuentes
dataset/     catálogo de juegos (precios)
agente/      estado + salida del agente mensual
.github/     workflows: scraping (diario), config-refresh (mensual)
```

## Requisitos

- **Rust** (con `cargo` en el PATH; `rustup` lo agrega). Si abrís terminal nueva, ya está; si no: `source ~/.cargo/env`.
- **Go** 1.23+.
- **Node 22** (usá `nvm`: `source ~/.nvm/nvm.sh && nvm use 22`). No uses el Node del sistema (viejo).

## Cómo levantarlo

Todo desde la raíz del repo:

```sh
make core          # compila el motor Rust → WASM (wasip1 + unknown). Necesario antes de web/tui.
make test-core     # tests del motor (Rust)
make test-services # tests de Go
make dev-web       # web en http://localhost:5173
make tui           # TUI en la consola (Bubble Tea)
make scrape        # actualiza precios del catálogo (Steam + Battle.net)
make agente        # corre el agente de config en local
```

Atajos: `make help` lista todo.

### Web

`make dev-web` levanta SvelteKit. Antes copia el WASM y la config a `web/static/` (`scripts/copiar-wasm.mjs`).

- `/` → **consulta**: buscás un juego/servicio y ves el desglose por medio (datos tal cual, sin editar).
- `/calculadora` → **calculadora custom**: ponés monto, moneda, régimen y provincia.

### TUI

`make tui`. Escribís para buscar; **↑↓** elegís, **Enter** abre/expande; las teclas están en el pie. Corre el mismo motor embebido.

## Conceptos clave

- **Régimen** (dos regímenes impositivos):
  - `gaming` → IVA 21% + IIBB, **Ganancias 0%** (~+23%).
  - `servicios_digitales` (Netflix, Spotify, ChatGPT, DeepSeek…) → IVA 21% + **Ganancias 30%** + IIBB (~+53%).
  - **Todo versionado** con `vigente_desde`/`vigente_hasta` en `config/tax-config.json`.
- **Medios de pago** (`config/wallets.json`): modelo **perfiles + emisores**. Cada emisor (banco/billetera) hereda un perfil (`banco_local`, `wallet_dolares`, `global_cripto`, …) y puede pisar `comision`/`cashback`/`fx`/`suscripcion_usd`. Los hosts los **expanden** a medios concretos.
  - `tipo` decide los impuestos: `tarjeta_local` (IVA+IIBB+Ganancias), `dolares_propios` (IVA+IIBB, evita Ganancias), `tarjeta_global` (sin percepciones locales).
- **Cotizaciones**: `dolarapi.com` (oficial, MEP, cripto, CCL) + `criptoya.com` (tasa real por wallet: ripio, belo, buenbit, lemon). Se traen en vivo.
- **Provincia**: IIBB es **provincial** (23 provincias + CABA). Se detecta por **IP** (`ipwho.is`) y se puede cambiar a mano.
- **On-demand**: productos prepago variables (DeepSeek) permiten **ingresar el monto** (mínimo configurable). El resto es precio fijo (solo lectura).
- **VAT de origen**: si el proveedor cobra un impuesto propio (ej. DeepSeek +6%), se aplica **automático** al monto neto que ingresás (`iva_origen`).
- **Anidado**: los productos con el mismo `grupo` (franquicia) se agrupan en los resultados (buscar `wow` → sub, ediciones…).

## Datos automáticos

- **Steam**: API pública (`appdetails`). AR cotiza en USD.
- **Battle.net**: la tienda da `401` a un UA normal, pero con **UA de crawler (Googlebot)** sirve el **structured data (`application/ld+json`)** con las ofertas y precios → se lee de ahí, **sin login**. Ver `services/internal/tiendas/tiendas.go`.
- Corre **solo, todos los días**, en `.github/workflows/scraping.yml`.

## Ramas y flujo

| Rama | Qué es |
|---|---|
| `main` | **producción** |
| `dev` | **integración** (donde se juntan las devs antes de prod) |
| `dev-ivan` | rama personal de Ivan |
| `dev-leo` | rama personal de Leo |

Flujo: se trabaja en tu rama personal (`dev-ivan` / `dev-leo`) → PR a `dev` → cuando está estable, `dev` → `main`.

## Convenciones

Ver **`AGENTS.md`** (es config ejecutable, no onboarding). Lo esencial:

- **TDD**: test que falla primero.
- **Conventional Commits** (`type(scope): subject`). Sin atribución de IA ni `Co-Authored-By`.
- **El motor no sabe nada de la pantalla**: no metas lógica de cálculo fuera de `core/`.
- **cgo**: solo el `data-worker` puede usar cgo; el resto `CGO_ENABLED=0`.
- **Secretos**: nunca en git. `.env` gitignoreado; keys de proveedores en el **keychain del SO** (`go run ./cmd/llave set deepseek`).

## Gotchas (cosas que sorprenden)

- **Node del sistema es viejo** → usá `nvm use 22`.
- **`cargo` no está en el PATH** hasta abrir terminal nueva o `source ~/.cargo/env`.
- **SvelteKit 3** usa `#lib` (no `$lib`).
- **dolarapi** devuelve JSON **mixto** (números + strings): parseá por campo, no como `map[string]float64`.
- **Battle.net** requiere el UA de crawler; un UA normal da 401.
- En **Mac**, la tecla `+` puede llegar como `=` en la TUI (por eso el binding acepta `+`, `=`, y flechas).

## Próximos pasos (roadmap)

1. **Fuentes FX declarativas** (`config/fuentes-fx.json` + extractor genérico): agregar wallets sin tocar código; que el agente las mantenga.
2. **Wallet** (libro de tokens append-only, transparente) + **admin**.
3. **Curar con fuente**: alícuotas IIBB provinciales (hoy algunas provisionales), comisiones/cashback por emisor, spread real de cada billetera.
