<script lang="ts">
	import { onMount } from 'svelte';
	import { cargarMotor, formatoArs, type Desglose, type Entrada } from '#lib/motor';
	import {
		aplicaMedio,
		buscar,
		cargarCatalogo,
		cargarConfig,
		cargarCotizaciones,
		cargarMedios,
		cargarServicios,
		nombreMedio,
		parametros,
		provincias,
		type Medio,
		type Servicio,
		type TaxConfig
	} from '#lib/datos';

	type Calculadora = (e: Entrada) => Promise<Desglose>;
	interface Calculo {
		medio: Medio;
		desglose: Desglose;
	}

	let motor: Calculadora | null = $state(null);
	let cfg: TaxConfig | null = $state(null);
	let servicios: Servicio[] = $state([]);
	let medios: Medio[] = $state([]);

	let provincia = $state('CABA');
	let query = $state('');
	let seleccion = $state<Servicio | null>(null);
	let monto = $state(0);
	let tasas = $state<Record<string, number>>({ oficial: 1500, mep: 1540 });
	let cotizadoEn = $state('');
	let calculos: Calculo[] = $state([]);
	let cargando = $state(true);
	let error = $state('');

	let resultados = $derived(query.trim() && !seleccion ? buscar(servicios, query) : []);
	let listaProvincias = $derived(cfg ? provincias(cfg) : []);
	let tienePrecio = $derived(seleccion?.precio != null);

	onMount(async () => {
		try {
			const [m, c, subs, meds, cat] = await Promise.all([
				cargarMotor(),
				cargarConfig(),
				cargarServicios(),
				cargarMedios(),
				cargarCatalogo()
			]);
			motor = m;
			cfg = c;
			servicios = [...cat, ...subs];
			medios = meds;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			cargando = false;
		}
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
		if (seleccion) await recalcular();
	}

	async function seleccionar(s: Servicio) {
		seleccion = s;
		query = s.nombre;
		monto = s.precio ?? 0;
		await recalcular();
	}

	function limpiar() {
		seleccion = null;
		calculos = [];
		query = '';
	}

	async function recalcular() {
		if (!motor || !cfg || !seleccion || seleccion.precio == null) {
			calculos = [];
			return;
		}
		const p = parametros(cfg, seleccion.regimen, provincia);
		const aplicables = medios.filter((m) => aplicaMedio(m, seleccion?.tienda, seleccion!.moneda));
		const out: Calculo[] = [];
		for (const m of aplicables) {
			try {
				const d = await motor({
					precio: { moneda: seleccion.moneda.toLowerCase(), monto },
					regimen: seleccion.regimen,
					parametros: p,
					medio: {
						nombre: nombreMedio(m),
						tipo: m.tipo,
						fx: m.fx,
						recargo_extra: m.comision ?? 0,
						cashback: m.cashback ?? 0
					},
					cotizaciones: tasas
				});
				out.push({ medio: m, desglose: d });
			} catch {
				// medio que no aplica (ej.: falta cotización) → se omite
			}
		}
		out.sort((a, b) => a.desglose.total_ars - b.desglose.total_ars);
		calculos = out;
	}
</script>

<main>
	<header>
		<h1>calculadora<span>.</span>impuestos<span>.</span>arg</h1>
		<p class="sub">Precio final de compras digitales, con impuestos y medios de pago. Argentina.</p>
	</header>

	{#if cargando}
		<p class="estado">Cargando motor fiscal…</p>
	{:else if error}
		<p class="estado error">Error: {error}</p>
	{:else}
		<section class="controles">
			<label>
				Provincia
				<select bind:value={provincia} onchange={recalcular}>
					{#each listaProvincias as p (p)}
						<option value={p}>{p}</option>
					{/each}
				</select>
			</label>
			<label>
				Dólar oficial
				<input type="number" bind:value={tasas.oficial} oninput={recalcular} />
			</label>
			<label>
				Dólar MEP
				<input type="number" bind:value={tasas.mep} oninput={recalcular} />
			</label>
			<div class="fx-estado">
				<button type="button" onclick={actualizarFx}>↻ actualizar</button>
				<span>{cotizadoEn ? `auto · ${cotizadoEn}` : 'manual (sin conexión)'}</span>
			</div>
		</section>

		<section class="buscador">
			<input
				type="text"
				placeholder="Buscá un juego o servicio… (ej. WoW Forever, Cyberpunk, ChatGPT)"
				bind:value={query}
				oninput={() => (seleccion = null)}
			/>
			{#if resultados.length}
				<ul class="resultados">
					{#each resultados as s (s.id)}
						<li>
							<button onclick={() => seleccionar(s)}>
								<strong>{s.nombre}</strong>
								<span class="meta">{s.tienda ?? s.regimen} · {s.precio != null ? `${s.precio} ${s.moneda}` : s.moneda}</span>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		{#if seleccion}
			<article class="card">
				<header class="card-head">
					<h2>{seleccion.nombre}</h2>
					<button class="limpiar" onclick={limpiar}>×</button>
				</header>
				<div class="precio-entrada">
					{#if tienePrecio}
						<span class="precio-dato">Precio de lista: <strong>{monto} {seleccion.moneda}</strong></span>
					{:else}
						<span class="precio-dato sin-dato">Sin precio de lista todavía (falta el dato)</span>
					{/if}
					<span class="regimen">{seleccion.regimen}{seleccion.tienda ? ` · ${seleccion.tienda}` : ''}</span>
				</div>

				{#if tienePrecio}
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
						{#each calculos as c, i (c.medio.id)}
							<tr class:mejor={i === 0}>
								<td>
									{i === 0 ? '▸ ' : ''}{nombreMedio(c.medio)}
									<span class="tipo">{c.medio.origen ?? ''} {c.medio.tipo}{c.medio.suscripcion_usd ? ` · US$${c.medio.suscripcion_usd}/mes` : ''}</span>
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

				{#if calculos.length}
					<p class="ahorro">
						Mejor hoy: <strong>{nombreMedio(calculos[0].medio)}</strong> — recargo efectivo
						{(calculos[0].desglose.tasa_efectiva * 100).toFixed(1)}%
					</p>
				{/if}
				{/if}
			</article>
		{/if}
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
	header h1 span {
		color: var(--magenta);
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
		margin-bottom: 1.5rem;
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
	.fx-estado button:hover {
		border-color: var(--cyan);
	}
	.fx-estado span {
		color: var(--dim);
		font-size: 0.72rem;
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
	.buscador input {
		width: 100%;
		padding: 0.8rem;
		font-size: 1rem;
	}
	.resultados {
		list-style: none;
		margin: 0.3rem 0 0;
		padding: 0;
		border: 1px solid var(--borde);
		border-radius: 4px;
		overflow: hidden;
	}
	.resultados button {
		width: 100%;
		display: flex;
		justify-content: space-between;
		background: var(--panel);
		border: none;
		border-bottom: 1px solid var(--borde);
		color: var(--texto);
		font: inherit;
		padding: 0.6rem 0.8rem;
		cursor: pointer;
		text-align: left;
	}
	.resultados button:hover {
		background: #142b1d;
	}
	.meta,
	.tipo {
		color: var(--dim);
		font-size: 0.75rem;
	}
	.tipo {
		display: block;
	}
	.card {
		margin-top: 2rem;
		border: 1px solid var(--borde);
		border-radius: 6px;
		background: var(--panel);
		padding: 1.25rem;
	}
	.card-head {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}
	.card-head h2 {
		color: var(--phosphor);
		margin: 0;
		font-size: 1.2rem;
	}
	.limpiar {
		background: none;
		border: none;
		color: var(--dim);
		font-size: 1.4rem;
		cursor: pointer;
	}
	.precio-entrada {
		display: flex;
		gap: 1.5rem;
		align-items: flex-end;
		margin: 1rem 0;
	}
	.regimen {
		color: var(--cyan);
		font-size: 0.8rem;
	}
	.precio-dato {
		color: var(--texto);
	}
	.precio-dato strong {
		color: var(--phosphor);
	}
	.sin-dato {
		color: var(--dim);
		font-style: italic;
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
	.ahorro {
		margin: 1rem 0 0;
		color: var(--texto);
	}
	.ahorro strong {
		color: var(--phosphor);
	}
</style>
