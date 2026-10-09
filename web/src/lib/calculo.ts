// Cálculo compartido: dado un monto + moneda + régimen + provincia, devuelve el
// desglose por cada medio aplicable, ordenado del más barato al más caro.
import type { Desglose, Entrada } from './motor';
import { aplicaMedio, nombreMedio, parametros, type Medio, type TaxConfig } from './datos';

export interface Calculo {
	medio: Medio;
	desglose: Desglose;
}

export type Calculadora = (e: Entrada) => Promise<Desglose>;

export interface OpcionesCalculo {
	monto: number;
	moneda: string;
	regimen: string;
	tienda?: string;
	provincia: string;
	tasas: Record<string, number>;
	/** IVA/percepción que cobra el PROVEEDOR de origen (ej. DeepSeek 6%). */
	ivaOrigen?: number;
}

export async function desgloses(
	motor: Calculadora,
	cfg: TaxConfig,
	medios: Medio[],
	opts: OpcionesCalculo
): Promise<Calculo[]> {
	let params;
	try {
		params = parametros(cfg, opts.regimen, opts.provincia);
	} catch {
		return [];
	}
	const aplicables = medios.filter((m) => aplicaMedio(m, opts.tienda, opts.moneda));
	// El monto que se cobra incluye el IVA/percepción de origen del proveedor.
	const montoBase = opts.monto * (1 + (opts.ivaOrigen ?? 0));
	const out: Calculo[] = [];
	for (const m of aplicables) {
		try {
			const d = await motor({
				precio: { moneda: opts.moneda.toLowerCase(), monto: montoBase },
				regimen: opts.regimen,
				parametros: params,
				medio: {
					nombre: nombreMedio(m),
					tipo: m.tipo,
					fx: m.fx,
					recargo_extra: m.comision ?? 0,
					cashback: m.cashback ?? 0
				},
				cotizaciones: opts.tasas
			});
			out.push({ medio: m, desglose: d });
		} catch {
			// medio que no aplica (ej.: falta cotización) → se omite
		}
	}
	out.sort((a, b) => a.desglose.total_ars - b.desglose.total_ars);
	return out;
}

/** Los medios con el MEJOR total (incluye empates dentro de una tolerancia). */
export function mejoresCalculos(calculos: Calculo[], tol = 0.001): Calculo[] {
	if (!calculos.length) return [];
	const mejor = calculos[0].desglose.total_ars;
	const limite = mejor + Math.max(0.5, mejor * tol);
	return calculos.filter((c) => c.desglose.total_ars <= limite);
}

/** Resumen "A · B · C (+N más)" de los mejores. */
export function resumenMejores(calculos: Calculo[], max = 4): string {
	const nombres = mejoresCalculos(calculos).map((c) => nombreMedio(c.medio));
	const mostrar = nombres.slice(0, max);
	const resto = nombres.length - mostrar.length;
	return mostrar.join(' · ') + (resto > 0 ? ` +${resto} más` : '');
}
