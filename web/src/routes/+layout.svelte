<script lang="ts">
	let { children } = $props();
</script>

{@render children()}

<style>
	:global(:root) {
		--bg: #0b0f0c;
		--phosphor: #63f5a1;
		--dim: #2e7d57;
		--cyan: #4ce0e0;
		--magenta: #ff5ca8;
		--texto: #cfeede;
		--panel: #0f160f;
		--borde: #1c3a28;
	}
	:global(html, body) {
		margin: 0;
		background: var(--bg);
		color: var(--texto);
		font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 15px;
		line-height: 1.5;
	}
	:global(*) {
		box-sizing: border-box;
	}
	:global(a) {
		color: var(--cyan);
	}
</style>
