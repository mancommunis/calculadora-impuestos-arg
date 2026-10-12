// Copia el motor WASM (build del core) y la config-as-code a static/.
// Se corre antes de dev/build. El wasm y config copiados están gitignoreados.
import { copyFileSync, existsSync, mkdirSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..'); // web/
const repo = join(raiz, '..'); // calculadora-impuestos-arg/

const wasm = join(repo, 'core/target/wasm32-unknown-unknown/release/calculadora_core.wasm');
if (!existsSync(wasm)) {
	console.error(
		`[copiar-wasm] falta ${wasm}\n` +
			`Corré 'make core' (o 'cargo build --release --target wasm32-unknown-unknown') primero.`
	);
	process.exit(1);
}
mkdirSync(join(raiz, 'static'), { recursive: true });
copyFileSync(wasm, join(raiz, 'static/motor.wasm'));

const cfgOrigen = join(repo, 'config');
const cfgDestino = join(raiz, 'static/config');
mkdirSync(cfgDestino, { recursive: true });
for (const f of readdirSync(cfgOrigen)) {
	if (f.endsWith('.json')) copyFileSync(join(cfgOrigen, f), join(cfgDestino, f));
}

// Catálogos (dataset), si existen.
const catOrigen = join(repo, 'dataset');
if (existsSync(catOrigen)) {
	mkdirSync(join(raiz, 'static/dataset'), { recursive: true });
	for (const f of readdirSync(catOrigen)) {
		if (f.endsWith('.json')) copyFileSync(join(catOrigen, f), join(raiz, 'static/dataset', f));
	}
}

console.log('[copiar-wasm] motor.wasm + config/*.json + dataset copiados a static/');
