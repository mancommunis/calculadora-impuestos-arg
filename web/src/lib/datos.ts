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
	nombre?: string;
	emisor?: string;
	origen?: string;
	tipo: string;
	fx: string;
	comision?: number;
	cashback?: number;
	suscripcion_usd?: number;
	acepta_en: string[];
	monedas?: string[];
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

/** Catálogo de juegos (dataset). Si no existe, devuelve vacío. */
export async function cargarCatalogo(): Promise<Servicio[]> {
	try {
		const data = await json<{ productos: Servicio[] }>('/dataset/catalogo.json');
		return data.productos ?? [];
	} catch {
		return [];
	}
}

export interface Perfil {
	tipo: string;
	origen: string;
	fx: string;
	comision?: number;
	cashback?: number;
	suscripcion_usd?: number;
	monedas?: string[];
	acepta_en?: string[];
}

interface Emisor {
	id: string;
	nombre: string;
	perfil: string;
	comision?: number;
	cashback?: number;
	suscripcion_usd?: number;
}

export async function cargarMedios(): Promise<Medio[]> {
	const data = await json<{ perfiles: Record<string, Perfil>; emisores: Emisor[] }>(
		'/config/wallets.json'
	);
	return data.emisores.map((e) => {
		const p = data.perfiles[e.perfil] ?? ({} as Perfil);
		return {
			id: e.id,
			nombre: e.nombre,
			emisor: e.nombre,
			origen: p.origen ?? '',
			tipo: p.tipo,
			fx: p.fx,
			comision: e.comision ?? p.comision ?? 0,
			cashback: e.cashback ?? p.cashback ?? 0,
			suscripcion_usd: e.suscripcion_usd ?? p.suscripcion_usd ?? 0,
			acepta_en: p.acepta_en ?? ['*'],
			monedas: p.monedas ?? []
		};
	});
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
	tasas: Record<string, number>;
	actualizado: string;
}

const FUENTES_FX: Record<string, string> = {
	oficial: 'https://dolarapi.com/v1/dolares/oficial',
	mep: 'https://dolarapi.com/v1/dolares/bolsa',
	cripto: 'https://dolarapi.com/v1/dolares/cripto',
	ccl: 'https://dolarapi.com/v1/dolares/contadoconliqui'
};

/** Cotizaciones en vivo desde dolarapi.com (público, CORS abierto). */
export async function cargarCotizaciones(): Promise<Cotizaciones> {
	const claves = Object.keys(FUENTES_FX);
	const respuestas = await Promise.allSettled(
		claves.map((k) => json<{ venta: number; fechaActualizacion?: string }>(FUENTES_FX[k]))
	);
	const tasas: Record<string, number> = {};
	let actualizado = '';
	respuestas.forEach((r, i) => {
		if (r.status === 'fulfilled') {
			tasas[claves[i]] = r.value.venta;
			if (!actualizado && r.value.fechaActualizacion) actualizado = r.value.fechaActualizacion;
		}
	});
	return { tasas, actualizado };
}

/** ¿El medio sirve para esta tienda y esta moneda del precio? */
export function aplicaMedio(m: Medio, tienda: string | undefined, moneda: string): boolean {
	const tiendaOK =
		!m.acepta_en?.length || m.acepta_en.includes('*') || (!!tienda && m.acepta_en.includes(tienda));
	if (!tiendaOK) return false;
	if (!m.monedas?.length) return true;
	return m.monedas.some((x) => x.toUpperCase() === moneda.toUpperCase());
}

export function nombreMedio(m: Medio): string {
	return m.nombre || m.id;
}
