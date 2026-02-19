<script lang="ts">
	import { base } from '$app/paths';
	import TableOfContents from '$lib/components/layout/TableOfContents.svelte';
	import type { DocPage } from '$lib/content/pipeline.js';

	let {
		data
	}: {
		data: {
			doc: DocPage;
			prev: { slug: string; title: string } | null;
			next: { slug: string; title: string } | null;
		};
	} = $props();
</script>

<svelte:head>
	<title>{data.doc.title} - SimConnect Go SDK</title>
	<meta name="description" content={data.doc.description} />
</svelte:head>

<div class="relative flex">
	<article class="prose min-w-0 max-w-4xl flex-1 p-6 lg:p-10">
		{@html data.doc.renderedContent}

		<!-- Prev/Next navigation -->
		{#if data.prev || data.next}
			<nav
				class="not-prose mt-12 flex items-stretch gap-4 border-t pt-6"
				style="border-color: var(--color-border);"
				aria-label="Guide navigation"
			>
				{#if data.prev}
					<a
						href="{base}/docs/{data.prev.slug}"
						class="group flex flex-1 flex-col rounded-lg border p-4 transition-colors"
						style="border-color: var(--color-border); background-color: var(--color-bg-secondary);"
					>
						<span class="text-xs uppercase tracking-wider" style="color: var(--color-text-muted);">
							Previous
						</span>
						<span class="mt-1 font-medium" style="color: var(--color-link);">
							&larr; {data.prev.title}
						</span>
					</a>
				{:else}
					<div class="flex-1"></div>
				{/if}
				{#if data.next}
					<a
						href="{base}/docs/{data.next.slug}"
						class="group flex flex-1 flex-col items-end rounded-lg border p-4 text-right transition-colors"
						style="border-color: var(--color-border); background-color: var(--color-bg-secondary);"
					>
						<span class="text-xs uppercase tracking-wider" style="color: var(--color-text-muted);">
							Next
						</span>
						<span class="mt-1 font-medium" style="color: var(--color-link);">
							{data.next.title} &rarr;
						</span>
					</a>
				{/if}
			</nav>
		{/if}
	</article>

	<aside class="toc-aside hidden pr-4 xl:block">
		<TableOfContents headings={data.doc.headings} />
	</aside>
</div>

<style>
	a.group:hover {
		border-color: var(--color-link) !important;
	}
</style>
