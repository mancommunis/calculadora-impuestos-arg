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
	const out: Calculo[] = [];
	for (const m of aplicables) {
		try {
			const d = await motor({
				precio: { moneda: opts.moneda.toLowerCase(), monto: opts.monto },
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
