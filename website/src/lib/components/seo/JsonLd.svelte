<script lang="ts">
	let { schema }: { schema: Record<string, unknown> } = $props();

	const safeJson = $derived(
		JSON.stringify(schema)
			.replace(/</g, '\\u003c')
			.replace(/>/g, '\\u003e')
			.replace(/&/g, '\\u0026')
	);
</script>

<svelte:head>
	{@html `<script type="application/ld+json">${safeJson}</script>`}
</svelte:head>
