<script lang="ts">
	let { schema }: { schema: Record<string, unknown> } = $props();
</script>

<svelte:head>
	{@html `<script type="application/ld+json">${JSON.stringify(schema)}</script>`}
</svelte:head>
