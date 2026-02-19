<script lang="ts">
	import { base } from '$app/paths';
	import type { DocMeta } from '$lib/types/index.js';

	let { data }: { data: { docs: DocMeta[] } } = $props();

	const sectionLabels: Record<string, string> = {
		client: 'Client / Engine',
		manager: 'Manager',
		events: 'Events',
		internals: 'Internals'
	};

	function groupBySection(docs: DocMeta[]) {
		const groups = new Map<string, DocMeta[]>();
		for (const doc of docs) {
			const list = groups.get(doc.section) ?? [];
			list.push(doc);
			groups.set(doc.section, list);
		}
		return groups;
	}

	const groups = $derived(groupBySection(data.docs));
</script>

<svelte:head>
	<title>Documentation - SimConnect Go SDK</title>
</svelte:head>

<div class="mx-auto max-w-3xl p-6 lg:p-10">
	<h1 class="mb-2 text-3xl font-bold" style="color: var(--color-text-primary);">Documentation</h1>
	<p class="mb-8" style="color: var(--color-text-secondary);">
		Complete reference for the SimConnect Go SDK.
	</p>

	{#each [...groups.entries()] as [section, docs]}
		<div class="mb-8">
			<h2
				class="mb-4 text-lg font-semibold uppercase tracking-wider"
				style="color: var(--color-text-muted);"
			>
				{sectionLabels[section] ?? section}
			</h2>
			<div class="space-y-3">
				{#each docs as doc}
					<a
						href="{base}/docs/{doc.slug}"
						class="block rounded-lg border p-4 transition-colors"
						style="border-color: var(--color-border); background-color: var(--color-bg-secondary);"
					>
						<h3 class="mb-1 font-medium" style="color: var(--color-link);">
							{doc.title}
						</h3>
						{#if doc.description}
							<p class="text-sm" style="color: var(--color-text-secondary);">
								{doc.description}
							</p>
						{/if}
					</a>
				{/each}
			</div>
		</div>
	{/each}
</div>
