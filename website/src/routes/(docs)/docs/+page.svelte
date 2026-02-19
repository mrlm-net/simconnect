<script lang="ts">
	import { base } from '$app/paths';
	import SeoHead from '$lib/components/seo/SeoHead.svelte';
	import { siteConfig } from '$lib/config/site.js';
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

	const externalLinks = [
		{
			section: 'Go Reference',
			links: [
				{
					title: 'pkg.go.dev',
					href: 'https://pkg.go.dev/github.com/mrlm-net/simconnect',
					description: 'Full API reference'
				}
			]
		},
		{
			section: 'SimConnect SDK',
			links: [
				{
					title: 'SDK Documentation',
					href: 'https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/SimConnect_SDK.htm',
					description: 'Official MSFS docs'
				},
				{
					title: 'Event IDs',
					href: 'https://docs.flightsimulator.com/html/Programming_Tools/Event_IDs/Event_IDs.htm',
					description: 'Key event reference'
				},
				{
					title: 'Simulation Variables',
					href: 'https://docs.flightsimulator.com/html/Programming_Tools/SimVars/Simulation_Variables.htm',
					description: 'SimVar reference'
				}
			]
		},
		{
			section: 'Project',
			links: [
				{
					title: 'GitHub Repository',
					href: 'https://github.com/mrlm-net/simconnect',
					description: 'Source code & issues'
				},
				{
					title: 'Examples',
					href: `${base}/examples`,
					description: '25+ example apps',
					internal: true
				}
			]
		}
	];
</script>

<SeoHead
	{siteConfig}
	title="Documentation - SimConnect Go SDK"
	description="Browse all SimConnect Go SDK documentation guides"
	path="/docs"
/>

<div class="flex">
	<div class="min-w-0 flex-1 p-6 pl-8 lg:p-10 lg:pl-12">
		<h1 class="mb-2 text-3xl font-bold" style="color: var(--color-text-primary);">
			Documentation
		</h1>
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

	<aside class="toc-aside hidden shrink-0 pr-4 xl:block">
		<nav class="py-6 pl-6 pr-6" style="min-width: 280px;" aria-label="External resources">
			{#each externalLinks as group}
				<div class="mb-5">
					<p
						class="mb-2 text-xs font-semibold uppercase tracking-wider"
						style="color: var(--color-text-muted);"
					>
						{group.section}
					</p>
					<ul class="space-y-1.5">
						{#each group.links as link}
							<li>
								<a
									href={link.href}
									target={link.internal ? undefined : '_blank'}
									rel={link.internal ? undefined : 'noopener noreferrer'}
									class="group block"
								>
									<span
										class="flex items-center gap-1 text-sm transition-colors"
										style="color: var(--color-link);"
									>
										{link.title}
										{#if !link.internal}
											<span class="text-xs" style="color: var(--color-text-muted);">&nearr;</span>
										{/if}
									</span>
									<span class="text-xs" style="color: var(--color-text-muted);">
										{link.description}
									</span>
								</a>
							</li>
						{/each}
					</ul>
				</div>
			{/each}
		</nav>
	</aside>
</div>
