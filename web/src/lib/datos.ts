// Carga la config-as-code (copiada a static/config) y arma el catálogo + búsqueda.

export interface TaxConfig {
	impuesto_pais: number;
	regimenes: Record<string, { iva: number; ganancias: number }>;
	iibb: Record<string, number>;
}

export interface Servicio {
	id: string;
	nombre: string;
	aliases?: string[];
	regimen: string;
	moneda: string;
	precio: number | null;
	tienda?: string;
}

export interface Medio {
	id: string;
	tipo: string;
	fx: string;
	recargo_extra: number;
	acepta_en: string[];
}

async function json<T>(ruta: string): Promise<T> {
	const r = await fetch(ruta);
	if (!r.ok) throw new Error(`no se pudo cargar ${ruta}`);
	return r.json() as Promise<T>;
}

export const cargarConfig = () => json<TaxConfig>('/config/tax-config.json');

export async function cargarServicios(): Promise<Servicio[]> {
	const data = await json<{ servicios: Servicio[] }>('/config/subscriptions.json');
	return data.servicios;
}

export async function cargarMedios(): Promise<Medio[]> {
	const data = await json<{ medios: Medio[] }>('/config/wallets.json');
	return data.medios;
}

export function provincias(cfg: TaxConfig): string[] {
	return Object.keys(cfg.iibb).sort();
}

export function parametros(cfg: TaxConfig, regimen: string, provincia: string) {
	const r = cfg.regimenes[regimen];
	if (!r) throw new Error(`régimen desconocido: ${regimen}`);
	const iibb = cfg.iibb[provincia];
	if (iibb == null) throw new Error(`provincia sin IIBB: ${provincia}`);
	return { iva: r.iva, ganancias: r.ganancias, iibb, impuesto_pais: cfg.impuesto_pais ?? 0 };
}

/** Búsqueda simple por nombre + aliases (substring, case-insensitive). */
export function buscar(servicios: Servicio[], q: string): Servicio[] {
	const t = q.trim().toLowerCase();
	if (!t) return [];
	return servicios
		.map((s) => {
			const campos = [s.nombre, s.id, ...(s.aliases ?? [])].map((x) => x.toLowerCase());
			let score = 0;
			for (const c of campos) {
				if (c === t) score = Math.max(score, 100);
				else if (c.startsWith(t)) score = Math.max(score, 60);
				else if (c.includes(t)) score = Math.max(score, 30);
			}
			return { s, score };
		})
		.filter((x) => x.score > 0)
		.sort((a, b) => b.score - a.score)
		.map((x) => x.s);
}

export interface Cotizaciones {
	oficial: number;
	mep: number;
	actualizado: string;
}

/** Cotizaciones en vivo desde dolarapi.com (público, CORS abierto). */
export async function cargarCotizaciones(): Promise<Cotizaciones> {
	const [oficial, mep] = await Promise.all([
		json<{ venta: number; fechaActualizacion: string }>('https://dolarapi.com/v1/dolares/oficial'),
		json<{ venta: number }>('https://dolarapi.com/v1/dolares/bolsa')
	]);
	return { oficial: oficial.venta, mep: mep.venta, actualizado: oficial.fechaActualizacion };
}
