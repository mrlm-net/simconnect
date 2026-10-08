<script lang="ts">
	let { glyph, size = 30 }: { glyph: string; size?: number } = $props();
</script>

<span class="mark" style="width: {size}px; height: {size}px; font-size: {Math.round(size * 0.42)}px;" aria-hidden="true">{glyph}</span>

<style>
	.mark {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		border-radius: 7px;
		background: var(--brand);
		color: var(--surface);
		font-family: var(--font-mono);
		font-weight: 600;
		letter-spacing: -0.04em;
	}
</style>
