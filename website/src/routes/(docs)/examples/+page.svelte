<script lang="ts">
	import Prism from 'prismjs';
	import 'prismjs/components/prism-clike';
	import 'prismjs/components/prism-go';
	import type { Example } from '$lib/content/examples.js';

	let {
		data
	}: {
		data: { examples: Example[]; categoryLabels: Record<string, string> };
	} = $props();

	let activeCategory = $state('all');
	let expandedSlug = $state('');

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

	function copyCode(code: string) {
		navigator.clipboard.writeText(code);
	}
</script>

<svelte:head>
	<title>Examples - SimConnect Go SDK</title>
	<meta
		name="description"
		content="Browse example applications for the SimConnect Go SDK."
	/>
</svelte:head>

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
			<button
				class="cursor-pointer rounded-full px-3 py-1 text-sm font-medium transition-colors"
				style="background-color: {active
					? 'var(--color-link)'
					: 'var(--color-bg-secondary)'}; color: {active
					? 'var(--color-bg-primary)'
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
							class="rounded-full px-2 py-0.5 text-xs"
							style="background-color: var(--color-bg-tertiary); color: var(--color-text-muted);"
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
								class="absolute top-2 right-2 rounded px-2 py-1 text-xs transition-colors"
								style="background-color: var(--color-bg-tertiary); color: var(--color-text-muted);"
								onclick={() => copyCode(example.code)}
							>
								Copy
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
