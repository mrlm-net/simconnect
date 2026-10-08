<script lang="ts">
	import type { SiteConfig } from '$lib/types/index.js';
	import BrandMark from './BrandMark.svelte';

	let { siteConfig }: { siteConfig: SiteConfig } = $props();
</script>

<footer class="site-footer px-6 py-8">
	<div class="flex flex-wrap items-center justify-between gap-x-6 gap-y-4 text-sm">
		<div class="flex flex-wrap items-center gap-3">
			<BrandMark glyph={siteConfig.glyph} size={22} />
			<span style="color: var(--text-2);">{siteConfig.title}</span>
			<a href="{siteConfig.repoUrl}/blob/main/LICENSE" target="_blank" rel="noopener noreferrer" class="pill">{siteConfig.licenseLabel}</a>
			<span style="color: var(--text-3);">&copy; {siteConfig.since}&ndash;{new Date().getFullYear()} mrlm-net</span>
		</div>
		<nav class="flex flex-wrap items-center gap-4" aria-label="Footer">
			{#each [['GitHub', ''], ['Releases', '/releases'], ['Issues', '/issues']] as [label, path] (label)}
				<a href="{siteConfig.repoUrl}{path}" target="_blank" rel="noopener noreferrer" class="footer-link">{label}</a>
			{/each}
		</nav>
	</div>
</footer>

<style>
	.site-footer {
		border-top: 1px solid var(--border);
		background: var(--bg);
	}
	.footer-link {
		color: var(--text-3);
	}
	.footer-link:hover {
		color: var(--text);
	}
</style>
