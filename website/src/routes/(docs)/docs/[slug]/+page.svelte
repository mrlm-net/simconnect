<script lang="ts">
	import TableOfContents from '$lib/components/layout/TableOfContents.svelte';
	import type { DocPage } from '$lib/content/pipeline.js';

	let { data }: { data: { doc: DocPage } } = $props();
</script>

<svelte:head>
	<title>{data.doc.title} - SimConnect Go SDK</title>
	<meta name="description" content={data.doc.description} />
</svelte:head>

<div class="flex">
	<article class="prose mx-auto min-w-0 max-w-4xl flex-1 p-6 lg:p-10">
		{@html data.doc.renderedContent}
	</article>

	<aside class="hidden shrink-0 pr-4 xl:block">
		<TableOfContents headings={data.doc.headings} />
	</aside>
</div>
