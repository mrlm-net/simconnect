<script lang="ts">
	import Prism from 'prismjs';
	import 'prismjs/components/prism-clike';
	import 'prismjs/components/prism-go';
	import SeoHead from '$lib/components/seo/SeoHead.svelte';
	import { siteConfig } from '$lib/config/site.js';
	import type { Example } from '$lib/content/examples.js';

	let {
		data
	}: {
		data: { examples: Example[]; categoryLabels: Record<string, string> };
	} = $props();

	let activeCategory = $state('all');
	let expandedSlug = $state('');
	let copiedSlug = $state('');

	const categories = $derived.by(() => {
		const cats = new Set(data.examples.map((e) => e.category));
		return ['all', ...cats];
	});

	const filtered = $derived.by(() => {
		if (activeCategory === 'all') return data.examples;
		return data.examples.filter((e) => e.category === activeCategory);
	});

	function highlight(code: string): string {
		const grammar = Prism.languages['go'];
		if (grammar) return Prism.highlight(code, grammar, 'go');
		return code.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
	}

	function toggle(slug: string) {
		expandedSlug = expandedSlug === slug ? '' : slug;
	}

	const categoryColors: Record<string, { bg: string; text: string }> = {
		basics: { bg: 'rgba(56, 139, 253, 0.15)', text: '#58a6ff' },
		data: { bg: 'rgba(188, 140, 255, 0.15)', text: '#bc8cff' },
		events: { bg: 'rgba(210, 153, 34, 0.15)', text: '#d2992a' },
		facilities: { bg: 'rgba(63, 185, 80, 0.15)', text: '#3fb950' },
		traffic: { bg: 'rgba(219, 109, 40, 0.15)', text: '#db6d28' },
		manager: { bg: 'rgba(57, 211, 204, 0.15)', text: '#39d3cc' }
	};

	function categoryColor(cat: string): { bg: string; text: string } {
		return categoryColors[cat] ?? { bg: 'var(--color-bg-tertiary)', text: 'var(--color-text-muted)' };
	}

	function copyCode(slug: string, code: string) {
		navigator.clipboard.writeText(code);
		copiedSlug = slug;
		setTimeout(() => (copiedSlug = ''), 2000);
	}
</script>

<SeoHead
	{siteConfig}
	title="Examples - SimConnect Go SDK"
	description="Browse example applications for the SimConnect Go SDK"
	path="/examples"
/>

<div class="p-6 pl-8 lg:p-10 lg:pl-12">
	<h1 class="mb-2 text-3xl font-bold" style="color: var(--color-text-primary);">Examples</h1>
	<p class="mb-6" style="color: var(--color-text-secondary);">
		Browse {data.examples.length} example applications covering connections, data reading, events,
		facilities, traffic, and the manager API.
	</p>

	<!-- Category filter -->
	<div class="mb-8 flex flex-wrap gap-2">
		{#each categories as cat (cat)}
			{@const active = activeCategory === cat}
			{@const colors = cat !== 'all' ? categoryColor(cat) : null}
			<button
				class="cursor-pointer rounded-full px-3 py-1 text-sm font-medium transition-colors"
				style="background-color: {active
					? 'var(--color-link)'
					: colors
						? colors.bg
						: 'var(--color-bg-secondary)'}; color: {active
					? 'var(--color-bg-primary)'
					: colors
						? colors.text
						: 'var(--color-text-secondary)'}; border: 1px solid {active
					? 'var(--color-link)'
					: 'var(--color-border)'};"
				onclick={() => (activeCategory = cat)}
			>
				{cat === 'all' ? 'All' : (data.categoryLabels[cat] ?? cat)}
			</button>
		{/each}
	</div>

	<!-- Examples list -->
	<div class="space-y-3">
		{#each filtered as example (example.slug)}
			{@const expanded = expandedSlug === example.slug}
			{@const badgeColors = categoryColor(example.category)}
			<div
				class="rounded-lg border"
				style="border-color: var(--color-border); background-color: var(--color-bg-secondary);"
			>
				<button
					class="flex w-full cursor-pointer items-center justify-between p-4 text-left"
					onclick={() => toggle(example.slug)}
				>
					<div>
						<h3 class="font-medium" style="color: var(--color-text-primary);">
							{example.title}
						</h3>
						{#if example.description}
							<p class="mt-0.5 text-sm" style="color: var(--color-text-secondary);">
								{example.description}
							</p>
						{/if}
					</div>
					<div class="flex items-center gap-2">
						<span
							class="rounded-full px-2 py-0.5 text-xs font-medium"
							style="background-color: {badgeColors.bg}; color: {badgeColors.text};"
						>
							{data.categoryLabels[example.category] ?? example.category}
						</span>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							class="shrink-0 transition-transform duration-150"
							class:rotate-180={expanded}
							style="color: var(--color-text-muted);"
						>
							<polyline points="6 9 12 15 18 9" />
						</svg>
					</div>
				</button>

				{#if expanded}
					<div class="border-t px-4 pb-4" style="border-color: var(--color-border);">
						<div class="relative mt-3">
							<button
								class="absolute top-2 right-2 flex items-center gap-1 rounded px-2 py-1 text-xs transition-colors"
								style="background-color: var(--color-bg-tertiary); color: {copiedSlug === example.slug ? '#3fb950' : 'var(--color-text-muted)'};"
								aria-label="Copy code"
								onclick={() => copyCode(example.slug, example.code)}
							>
								{#if copiedSlug === example.slug}
									<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" /></svg>
									Copied
								{:else}
									<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
									Copy
								{/if}
							</button>
							<pre
								class="overflow-x-auto rounded-lg border p-4"
								style="background-color: var(--color-bg-code); border-color: var(--color-border);"
							><code class="language-go" style="font-family: var(--font-mono); font-size: 0.8125rem; line-height: 1.7;">{@html highlight(example.code)}</code></pre>
						</div>
						<div class="mt-3 flex gap-3">
							<a
								href="https://github.com/mrlm-net/simconnect/tree/main/examples/{example.slug}"
								target="_blank"
								rel="noopener noreferrer"
								class="text-sm"
								style="color: var(--color-link);"
							>
								View on GitHub &nearr;
							</a>
						</div>
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>
