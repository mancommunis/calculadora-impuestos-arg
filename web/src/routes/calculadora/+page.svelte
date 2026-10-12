<script lang="ts">
	import { onMount } from 'svelte';
	import { cargarMotor, formatoArs } from '#lib/motor';
	import { desgloses, mejoresCalculos, resumenMejores, type Calculo, type Calculadora } from '#lib/calculo';
	import {
		cargarConfig,
		cargarCotizaciones,
		cargarMedios,
		detectarProvincia,
		nombreMedio,
		provincias,
		type Medio,
		type TaxConfig
	} from '#lib/datos';

	let motor: Calculadora | null = $state(null);
	let cfg: TaxConfig | null = $state(null);
	let medios: Medio[] = $state([]);
	let tasas = $state<Record<string, number>>({ oficial: 1500, mep: 1540 });
	let cotizadoEn = $state('');

	let monto = $state(10);
	let moneda = $state('USD');
	let regimen = $state('servicios_digitales');
	let provincia = $state('CABA');

	let calculos: Calculo[] = $state([]);
	let cargando = $state(true);
	let error = $state('');

	let listaProvincias = $derived(cfg ? provincias(cfg) : []);
	let resumen = $derived(resumenMejores(calculos));
	let idsMejores = $derived(new Set(mejoresCalculos(calculos).map((c) => c.medio.id)));

	onMount(async () => {
		try {
			const [m, c, meds] = await Promise.all([cargarMotor(), cargarConfig(), cargarMedios()]);
			motor = m;
			cfg = c;
			medios = meds;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			cargando = false;
		}
		const detectada = await detectarProvincia();
		if (cfg && detectada && listaProvincias.includes(detectada)) provincia = detectada;
		await actualizarFx();
	});

	async function actualizarFx() {
		try {
			const c = await cargarCotizaciones();
			if (Object.keys(c.tasas).length) tasas = c.tasas;
			cotizadoEn = c.actualizado ? new Date(c.actualizado).toLocaleString('es-AR') : '';
		} catch {
			cotizadoEn = '';
		}
		await recalcular();
	}

	async function recalcular() {
		if (!motor || !cfg) {
			calculos = [];
			return;
		}
		calculos = await desgloses(motor, cfg, medios, { monto, moneda, regimen, provincia, tasas });
	}
</script>

<main>
	<header>
		<h1>calculadora<span>.</span>custom</h1>
		<p class="sub">
			Poné un monto y mirá el costo final por medio de pago. <a href="/">← volver a consulta</a>
		</p>
	</header>

	{#if cargando}
		<p class="estado">Cargando motor fiscal…</p>
	{:else if error}
		<p class="estado error">Error: {error}</p>
	{:else}
		<section class="controles">
			<label>
				Monto
				<input type="number" bind:value={monto} oninput={recalcular} min="0" step="1" />
			</label>
			<label>
				Moneda
				<select bind:value={moneda} onchange={recalcular}>
					<option value="USD">USD</option>
					<option value="ARS">ARS</option>
				</select>
			</label>
			<label>
				Régimen
				<select bind:value={regimen} onchange={recalcular}>
					<option value="servicios_digitales">servicios digitales (Netflix, DeepSeek…)</option>
					<option value="gaming">gaming (juegos)</option>
				</select>
			</label>
			<label>
				Provincia
				<select bind:value={provincia} onchange={recalcular}>
					{#each listaProvincias as p (p)}
						<option value={p}>{p}</option>
					{/each}
				</select>
			</label>
			<div class="fx-estado">
				<button type="button" onclick={actualizarFx}>↻ actualizar dólar</button>
				<span>{cotizadoEn ? `auto · ${cotizadoEn}` : 'manual (sin conexión)'}</span>
			</div>
		</section>

		<p class="ayuda">
			Tip: DeepSeek cobra mínimo <strong>US$2</strong> (+6% VAT = US$2,12) y es prepago on-demand.
		</p>

		{#if calculos.length}
			<p class="ahorro">
				Los mejores hoy: <strong>{resumen}</strong> — costo
				<strong>{formatoArs(calculos[0].desglose.total_ars)}</strong>
				({((calculos[0].desglose.total_ars / calculos[0].desglose.base_ars - 1) * 100).toFixed(1)}% sobre la base)
			</p>
		{/if}

		<table>
			<thead>
				<tr>
					<th>Medio de pago</th>
					<th class="num">Base ARS</th>
					<th class="num">IVA</th>
					<th class="num">IIBB</th>
					<th class="num">Ganancias</th>
					<th class="num">Cashback</th>
					<th class="num">Total</th>
				</tr>
			</thead>
			<tbody>
				{#each calculos as c (c.medio.id)}
					<tr class:mejor={idsMejores.has(c.medio.id)}>
						<td>
							{idsMejores.has(c.medio.id) ? '▸ ' : ''}{nombreMedio(c.medio)}
							<span class="tipo">{c.medio.origen ?? ''} {c.medio.tipo}</span>
						</td>
						<td class="num">{formatoArs(c.desglose.base_ars)}</td>
						<td class="num">{formatoArs(c.desglose.iva)}</td>
						<td class="num">{formatoArs(c.desglose.iibb)}</td>
						<td class="num">{formatoArs(c.desglose.ganancias)}</td>
						<td class="num">{c.desglose.cashback ? '-' + formatoArs(c.desglose.cashback) : '—'}</td>
						<td class="num total">{formatoArs(c.desglose.total_ars)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</main>

<style>
	main {
		max-width: 1000px;
		margin: 0 auto;
		padding: 2rem 1rem 4rem;
	}
	header h1 {
		color: var(--phosphor);
		font-size: 1.6rem;
		margin: 0;
		letter-spacing: -0.5px;
	}
	.sub {
		color: var(--dim);
		margin: 0.25rem 0 2rem;
	}
	.estado {
		color: var(--dim);
	}
	.error {
		color: var(--magenta);
	}
	.controles {
		display: flex;
		gap: 1.5rem;
		flex-wrap: wrap;
		align-items: flex-end;
		margin-bottom: 1rem;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		color: var(--dim);
		font-size: 0.8rem;
	}
	input,
	select {
		background: var(--panel);
		border: 1px solid var(--borde);
		color: var(--texto);
		font: inherit;
		padding: 0.5rem 0.6rem;
		border-radius: 4px;
	}
	input:focus,
	select:focus {
		outline: none;
		border-color: var(--phosphor);
	}
	.fx-estado {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}
	.fx-estado button {
		background: var(--panel);
		border: 1px solid var(--borde);
		color: var(--cyan);
		font: inherit;
		padding: 0.5rem 0.7rem;
		border-radius: 4px;
		cursor: pointer;
	}
	.ayuda {
		color: var(--dim);
		font-size: 0.82rem;
		margin: 0.5rem 0 1.25rem;
	}
	.ayuda strong {
		color: var(--cyan);
	}
	.ahorro {
		color: var(--texto);
	}
	.ahorro strong {
		color: var(--phosphor);
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.85rem;
	}
	th,
	td {
		padding: 0.5rem 0.6rem;
		border-bottom: 1px solid var(--borde);
	}
	th {
		color: var(--dim);
		text-align: left;
		font-weight: 400;
	}
	.num {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	.total {
		color: var(--phosphor);
	}
	tr.mejor td {
		color: var(--phosphor);
	}
	.tipo {
		display: block;
		color: var(--dim);
		font-size: 0.75rem;
	}
</style>
