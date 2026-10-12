// Carga la config-as-code (copiada a static/config) y arma el catálogo + búsqueda.

export interface TaxConfig {
	impuesto_pais: number;
	regimenes: Record<string, { iva: number; ganancias: number }>;
	iibb: Record<string, number>;
}

export interface Servicio {
	id: string;
	nombre: string;
	grupo?: string;
	subgrupo?: string;
	aliases?: string[];
	regimen: string;
	moneda: string;
	precio: number | null;
	on_demand?: boolean;
	minimo?: number;
	iva_origen?: number;
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

/** Catálogo de juegos (dataset): catalogo.json + battlenet.json. */
export async function cargarCatalogo(): Promise<Servicio[]> {
	let out: Servicio[] = [];
	for (const f of ['/dataset/catalogo.json', '/dataset/battlenet.json']) {
		try {
			const data = await json<{ productos: Servicio[] }>(f);
			out = out.concat(data.productos ?? []);
		} catch {
			// archivo ausente → seguimos
		}
	}
	return out;
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
	fx?: string;
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
			fx: e.fx ?? p.fx,
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

function puntajeCampo(campo: string, t: string): number {
	const c = campo.toLowerCase();
	if (c === t) return 100;
	if (c.startsWith(t)) return 70;
	if (c.includes(t)) return 45;
	if (t.length >= 3 && subsecuencia(t, c)) return 20;
	return 0;
}

function subsecuencia(q: string, c: string): boolean {
	let i = 0;
	let last = -1;
	let gaps = 0;
	for (let j = 0; j < c.length && i < q.length; j++) {
		if (c[j] === q[i]) {
			if (last >= 0) gaps += j - last - 1;
			last = j;
			i++;
		}
	}
	return i === q.length && gaps <= 4;
}

/** Búsqueda por nombre + aliases (substring + fuzzy, tolera typos). */
export function buscar(servicios: Servicio[], q: string): Servicio[] {
	const t = q.trim().toLowerCase();
	if (!t) return [];
	return servicios
		.map((s) => {
			let score = 0;
			for (const campo of [s.nombre, s.id, ...(s.aliases ?? [])]) {
				score = Math.max(score, puntajeCampo(campo, t));
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

const FUENTES_DOLARAPI: Record<string, string> = {
	oficial: 'https://dolarapi.com/v1/dolares/oficial',
	mep: 'https://dolarapi.com/v1/dolares/bolsa',
	cripto: 'https://dolarapi.com/v1/dolares/cripto',
	ccl: 'https://dolarapi.com/v1/dolares/contadoconliqui'
};

// Wallets que cotizan en CriptoYa (USDT/ARS): su tasa real, automática.
const EXCHANGES_CRIPTOYA = ['ripio', 'belo', 'buenbit', 'lemon'];

/** Cotizaciones en vivo: dolarapi (oficial/mep/cripto/ccl) + CriptoYa por wallet. */
export async function cargarCotizaciones(): Promise<Cotizaciones> {
	const tasas: Record<string, number> = {};
	let actualizado = '';

	await Promise.allSettled([
		...Object.entries(FUENTES_DOLARAPI).map(async ([k, u]) => {
			const d = await json<{ venta: number; fechaActualizacion?: string }>(u);
			tasas[k] = d.venta;
			if (k === 'oficial' && d.fechaActualizacion) actualizado = d.fechaActualizacion;
		}),
		...EXCHANGES_CRIPTOYA.map(async (ex) => {
			const d = await json<{ ask: number }>(`https://criptoya.com/api/${ex}/usdt/ars/1`);
			if (d.ask) tasas[ex] = d.ask;
		})
	]);

	// Fallback: wallet sin tasa propia -> usa la cripto.
	for (const ex of EXCHANGES_CRIPTOYA) {
		if (!tasas[ex] && tasas.cripto) tasas[ex] = tasas.cripto;
	}

	return { tasas, actualizado };
}

export interface Subgrupo {
	nombre: string;
	items: Servicio[];
}
export interface Grupo {
	clave: string;
	nombre: string;
	total: number;
	subgrupos: Subgrupo[];
}

/** Busca y arma el árbol grupo → subgrupo → items. */
export function buscarArbol(servicios: Servicio[], q: string): Grupo[] {
	const encontrados = buscar(servicios, q);
	const gmap = new Map<string, { nombre: string; sub: Map<string, Servicio[]> }>();
	for (const s of encontrados) {
		const gc = s.grupo ?? s.id;
		const gn = s.grupo ?? s.nombre;
		if (!gmap.has(gc)) gmap.set(gc, { nombre: gn, sub: new Map() });
		const sub = gmap.get(gc)!.sub;
		const sc = s.subgrupo ?? '';
		if (!sub.has(sc)) sub.set(sc, []);
		sub.get(sc)!.push(s);
	}
	return [...gmap.entries()].map(([clave, v]) => {
		const subgrupos = [...v.sub.entries()].map(([nombre, items]) => ({ nombre, items }));
		return {
			clave,
			nombre: v.nombre,
			total: subgrupos.reduce((a, b) => a + b.items.length, 0),
			subgrupos
		};
	});
}

// --- geolocalización por IP (default de provincia) ---

const CLAVES_PROVINCIA = [
	'CABA',
	'PBA',
	'Catamarca',
	'Chaco',
	'Chubut',
	'Cordoba',
	'Corrientes',
	'Entre Rios',
	'Formosa',
	'Jujuy',
	'La Pampa',
	'La Rioja',
	'Mendoza',
	'Misiones',
	'Neuquen',
	'Rio Negro',
	'Salta',
	'San Juan',
	'San Luis',
	'Santa Cruz',
	'Santa Fe',
	'Santiago del Estero',
	'Tierra del Fuego',
	'Tucuman'
];

function normalizar(s: string): string {
	return s
		.normalize('NFD')
		.replace(/[\u0300-\u036f]/g, '')
		.toLowerCase()
		.replace(/\s+(province|provincia)$/, '')
		.trim();
}

/** Mapea el nombre de región de un geo-IP a nuestras claves de provincia. */
export function mapearProvincia(region: string): string | null {
	const n = normalizar(region);
	if (!n) return null;
	if (n.includes('capital federal') || (n.includes('ciudad') && n.includes('buenos aires')))
		return 'CABA';
	if (n.includes('buenos aires')) return 'PBA';
	for (const clave of CLAVES_PROVINCIA) {
		if (normalizar(clave) === n) return clave;
	}
	return null;
}

/** Detecta la provincia del visitante por IP (ipwho.is, sin pedir permisos). */
export async function detectarProvincia(): Promise<string | null> {
	try {
		const r = await fetch('https://ipwho.is/');
		if (!r.ok) return null;
		const d = (await r.json()) as { country_code?: string; region?: string; city?: string };
		if (d.country_code !== 'AR') return null;
		return mapearProvincia(d.region ?? d.city ?? '');
	} catch {
		return null;
	}
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
