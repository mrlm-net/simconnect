<script lang="ts">
	import SeoHead from '$lib/components/seo/SeoHead.svelte';
	import JsonLd from '$lib/components/seo/JsonLd.svelte';
	import { siteConfig } from '$lib/config/site.js';
	import type { ChangelogRelease } from '$lib/types/index.js';

	let {
		data
	}: {
		data: { releases: ChangelogRelease[] };
	} = $props();

	function formatDate(iso: string): string {
		const d = new Date(iso);
		return d.toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'long',
			day: 'numeric'
		});
	}

	function minorKey(tag: string): string {
		const m = tag.match(/^v?(\d+\.\d+)/);
		return m ? `v${m[1]}` : 'other';
	}

	// Group releases by minor version, preserve insertion order after sort
	const grouped = $derived.by(() => {
		const map = new Map<string, ChangelogRelease[]>();
		for (const r of data.releases) {
			const key = minorKey(r.tag);
			const arr = map.get(key) ?? [];
			arr.push(r);
			map.set(key, arr);
		}
		// Sort keys descending by major then minor number
		const sorted = [...map.keys()].sort((a, b) => {
			const [aMaj, aMin] = a.replace('v', '').split('.').map(Number);
			const [bMaj, bMin] = b.replace('v', '').split('.').map(Number);
			return bMaj - aMaj || bMin - aMin;
		});
		return { map, keys: sorted };
	});

	let activeTab = $state('');

	// Set default tab to latest minor once data is available
	$effect(() => {
		if (!activeTab && grouped.keys.length > 0) {
			activeTab = grouped.keys[0];
		}
	});

	const activeReleases = $derived(grouped.map.get(activeTab) ?? []);
</script>

<SeoHead
	{siteConfig}
	title="Changelog - SimConnect Go SDK"
	description="Release history for the SimConnect Go SDK."
	path="/changelog"
/>
<JsonLd
	schema={{
		'@context': 'https://schema.org',
		'@type': 'WebPage',
		name: 'Changelog',
		description: 'Release history for the SimConnect Go SDK.',
		url: `${siteConfig.url}${siteConfig.basePath}/changelog`
	}}
/>

<div class="px-8 py-6 lg:px-12 lg:py-10">
	<h1 class="mb-2 text-3xl font-bold" style="color: var(--color-text-primary);">Changelog</h1>
	<p class="mb-6" style="color: var(--color-text-secondary);">
		Release history for the SimConnect Go SDK.
	</p>

	{#if data.releases.length === 0}
		<div
			class="rounded-lg border p-8 text-center"
			style="border-color: var(--color-border); background-color: var(--color-bg-secondary);"
		>
			<p class="mb-2" style="color: var(--color-text-secondary);">No releases found.</p>
			<a
				href="{siteConfig.repoUrl}/releases"
				target="_blank"
				rel="noopener noreferrer"
				class="text-sm"
				style="color: var(--color-link);"
			>
				View releases on GitHub &nearr;
			</a>
		</div>
	{:else}
		<!-- Tab bar -->
		<div
			class="mb-8 flex gap-0 overflow-x-auto border-b"
			style="border-color: var(--color-border);"
			role="tablist"
		>
			{#each grouped.keys as key (key)}
				{@const isActive = key === activeTab}
				<button
					role="tab"
					aria-selected={isActive}
					onclick={() => (activeTab = key)}
					class="whitespace-nowrap px-4 py-2.5 text-sm font-medium transition-colors"
					style={isActive
						? 'color: var(--color-text-primary); border-bottom: 2px solid var(--color-border-active); margin-bottom: -1px;'
						: 'color: var(--color-text-muted); border-bottom: 2px solid transparent; margin-bottom: -1px;'}
				>
					{key}.x
				</button>
			{/each}
		</div>

		<!-- Releases for active tab -->
		<div class="space-y-0" role="tabpanel">
			{#each activeReleases as release, i (release.tag)}
				{#if i > 0}
					<hr class="my-8" style="border-color: var(--color-border);" />
				{/if}
				<article>
					<div class="mb-4 flex flex-wrap items-center gap-3">
						<a
							href={release.url}
							target="_blank"
							rel="noopener noreferrer"
							class="text-xl font-semibold transition-colors"
							style="color: var(--color-link);"
						>
							{release.tag}
						</a>
						{#if release.name !== release.tag}
							<span class="text-lg font-medium" style="color: var(--color-text-primary);">
								&mdash; {release.name}
							</span>
						{/if}
						{#if release.prerelease}
							<span
								class="rounded-full px-2 py-0.5 text-xs font-medium"
								style="background-color: rgba(210, 153, 34, 0.15); color: #d2992a;"
							>
								pre-release
							</span>
						{/if}
					</div>
					<time
						class="mb-4 block text-sm"
						datetime={release.date}
						style="color: var(--color-text-muted);"
					>
						{formatDate(release.date)}
					</time>
					{#if release.renderedBody}
						<div class="prose max-w-none">
							{@html release.renderedBody}
						</div>
					{/if}
				</article>
			{/each}
		</div>
	{/if}
</div>
