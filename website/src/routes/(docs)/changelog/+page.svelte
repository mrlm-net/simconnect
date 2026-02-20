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
	<p class="mb-8" style="color: var(--color-text-secondary);">
		Release history for the SimConnect Go SDK.
	</p>

	{#if data.releases.length === 0}
		<div
			class="rounded-lg border p-8 text-center"
			style="border-color: var(--color-border); background-color: var(--color-bg-secondary);"
		>
			<p class="mb-2" style="color: var(--color-text-secondary);">
				No releases found.
			</p>
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
		<div class="space-y-0">
			{#each data.releases as release, i (release.tag)}
				{#if i > 0}
					<hr
						class="my-8"
						style="border-color: var(--color-border);"
					/>
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
							<span
								class="text-lg font-medium"
								style="color: var(--color-text-primary);"
							>
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
