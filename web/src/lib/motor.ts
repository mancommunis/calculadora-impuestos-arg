// Loader del motor fiscal (Rust → WASM) con la WebAssembly API, sin glue.
// Usa el mismo ABI plano que Go: cfg_alloc / cfg_calcular_json / cfg_free.

export interface ParametrosFiscales {
	iva: number;
	ganancias: number;
	iibb: number;
	impuesto_pais: number;
}

export interface Entrada {
	precio: { moneda: string; monto: number };
	regimen: string;
	parametros: ParametrosFiscales;
	medio: { nombre: string; tipo: string; fx: string; recargo_extra?: number; cashback?: number };
	cotizaciones: Record<string, number>;
}

export interface Desglose {
	regimen: string;
	moneda_precio: string;
	monto_precio: number;
	fx_nombre: string | null;
	fx_valor: number | null;
	base_ars: number;
	iva: number;
	iibb: number;
	ganancias: number;
	impuesto_pais: number;
	recargo_extra: number;
	cashback: number;
	total_ars: number;
	tasa_efectiva: number;
}

type Calculadora = (entrada: Entrada) => Promise<Desglose>;

let cache: Promise<Calculadora> | null = null;

export function cargarMotor(): Promise<Calculadora> {
	if (!cache) cache = instanciar();
	return cache;
}

async function instanciar(): Promise<Calculadora> {
	const res = await fetch('/motor.wasm');
	const bytes = await res.arrayBuffer();
	const { instance } = await WebAssembly.instantiate(bytes, {});

	const { memory, cfg_alloc, cfg_calcular_json, cfg_free } = instance.exports as unknown as {
		memory: WebAssembly.Memory;
		cfg_alloc: (len: number) => number;
		cfg_calcular_json: (ptr: number, len: number) => number;
		cfg_free: (ptr: number, len: number) => void;
	};

	const enc = new TextEncoder();
	const dec = new TextDecoder();

	return async (entrada: Entrada): Promise<Desglose> => {
		const data = enc.encode(JSON.stringify(entrada));
		const ptr = cfg_alloc(data.length);
		new Uint8Array(memory.buffer).set(data, ptr);
		const outPtr = cfg_calcular_json(ptr, data.length);
		const n = new DataView(memory.buffer).getUint32(outPtr, true);
		const out = dec.decode(new Uint8Array(memory.buffer, outPtr + 4, n));
		cfg_free(ptr, data.length);
		cfg_free(outPtr, 4 + n);

		const json = JSON.parse(out);
		if (json.error) throw new Error(json.error);
		return json as Desglose;
	};
}

export function formatoArs(n: number): string {
	return new Intl.NumberFormat('es-AR', {
		style: 'currency',
		currency: 'ARS',
		maximumFractionDigits: 0
	}).format(n);
}
