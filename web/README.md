# web (SvelteKit)

Web estándar: buscador + card de detalle con desglose fiscal por medio de pago y comparación entre tiendas.

- **Svelte 5** (runes) + SvelteKit, deploy **Vercel**.
- Carga el **motor WASM** del `core` en el browser (WebAssembly API, sin glue) — el mismo motor que usa Go.
- La config-as-code (`config/*.json`) y el motor (`motor.wasm`) se copian a `static/` antes de dev/build (`scripts/copiar-wasm.mjs`); están gitignoreados.

## Comandos

```sh
npm run wasm    # copia motor.wasm + config a static/ (requiere 'make core')
npm run dev     # dev server (corre 'wasm' solo)
npm run build   # build de producción (corre 'wasm' solo)
```

## Fase

Fase 3. Próximo: fuzzy real (Fuse.js), dataset desde `data/`, estética CRT.
