<script lang="ts">
	import { base } from '$app/paths';
	import TableOfContents from '$lib/components/layout/TableOfContents.svelte';
	import SeoHead from '$lib/components/seo/SeoHead.svelte';
	import JsonLd from '$lib/components/seo/JsonLd.svelte';
	import { siteConfig } from '$lib/config/site.js';
	import type { DocPage } from '$lib/content/types.js';

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

<SeoHead
	{siteConfig}
	title="{data.doc.title} - SimConnect Go SDK"
	description={data.doc.description}
	path="/docs/{data.doc.slug}"
	type="article"
/>
<JsonLd
	schema={{
		'@context': 'https://schema.org',
		'@type': 'TechArticle',
		headline: data.doc.title,
		description: data.doc.description,
		url: `${siteConfig.url}${siteConfig.basePath}/docs/${data.doc.slug}`
	}}
/>

<div class="flex justify-center">
	<article class="doc prose min-w-0 flex-1 px-6 py-10 sm:px-10 lg:py-14">
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
						class="card group flex flex-1 flex-col px-4 py-3 "
					>
						<span class="eyebrow">
							Previous
						</span>
						<span class="mt-1 font-medium" style="color: var(--text);">
							&larr; {data.prev.title}
						</span>
					</a>
				{:else}
					<div class="flex-1"></div>
				{/if}
				{#if data.next}
					<a
						href="{base}/docs/{data.next.slug}"
						class="card group flex flex-1 flex-col items-end px-4 py-3 text-right "
					>
						<span class="eyebrow">
							Next
						</span>
						<span class="mt-1 font-medium" style="color: var(--text);">
							{data.next.title} &rarr;
						</span>
					</a>
				{/if}
			</nav>
		{/if}
	</article>

	<aside class="toc-aside hidden shrink-0 pr-4 xl:block">
		<TableOfContents headings={data.doc.headings} />
	</aside>
</div>

<style>
	a.group:hover {
		border-color: var(--border-strong) !important;
	}
	.doc {
		max-width: 52rem;
	}
</style>
