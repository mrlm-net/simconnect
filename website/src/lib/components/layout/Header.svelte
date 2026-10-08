<script lang="ts">
	import { base } from '$app/paths';
	import { page } from '$app/state';
	import type { SiteConfig } from '$lib/types/index.js';
	import BrandMark from './BrandMark.svelte';
	import ThemeSwitch from './ThemeSwitch.svelte';

	let {
		siteConfig,
		onToggleSidebar,
		showMenuButton = true
	}: { siteConfig: SiteConfig; onToggleSidebar: () => void; showMenuButton?: boolean } =
		$props();

	// The longest matching link is the active one (Docs vs a page under /docs).
	const active = $derived.by(() => {
		const path = page.url.pathname;
		let best = '';
		for (const l of siteConfig.nav) {
			const href = `${base}${l.match ?? l.href}`;
			if (path.startsWith(href) && href.length > best.length) best = href;
		}
		return best;
	});
</script>

<header class="site-header fixed top-0 right-0 left-0 z-40 flex h-16 items-center justify-between gap-3 px-4">
	<div class="flex min-w-0 items-center gap-2">
		{#if showMenuButton}
			<button class="icon-btn md:hidden" onclick={onToggleSidebar} aria-label="Toggle navigation">
				<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" aria-hidden="true">
					<path d="M4 6h16M4 12h16M4 18h16" />
				</svg>
			</button>
		{/if}
		<a href="{base}/" class="flex min-w-0 items-center gap-2.5 font-semibold tracking-tight" style="color: var(--text);">
			<BrandMark glyph={siteConfig.glyph} />
			<span class="truncate">{siteConfig.title}</span>
		</a>
	</div>

	<div class="flex items-center gap-1 sm:gap-2">
		<nav class="hidden items-center gap-1 sm:flex" aria-label="Main">
			{#each siteConfig.nav as link (link.href)}
				{@const on = active === `${base}${link.match ?? link.href}`}
				<a href="{base}{link.href}" class="nav-link" class:on aria-current={on ? 'page' : undefined}>{link.title}</a>
			{/each}
		</nav>
		<ThemeSwitch />
		<a href={siteConfig.repoUrl} target="_blank" rel="noopener noreferrer" class="icon-btn" aria-label="GitHub repository">
			<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
				<path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
			</svg>
		</a>
	</div>
</header>

<style>
	.site-header {
		background: color-mix(in srgb, var(--surface) 88%, transparent);
		backdrop-filter: saturate(1.4) blur(10px);
		border-bottom: 1px solid var(--border);
	}
	.nav-link {
		padding: 0.4rem 0.7rem;
		border-radius: 6px;
		font-size: 0.875rem;
		color: var(--text-2);
		transition: color 120ms, background-color 120ms;
	}
	.nav-link:hover {
		color: var(--text);
		background: var(--surface-2);
	}
	.nav-link.on {
		color: var(--text);
		font-weight: 500;
		background: var(--surface-3);
	}
	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		border-radius: 6px;
		color: var(--text-2);
		cursor: pointer;
	}
	.icon-btn:hover {
		color: var(--text);
		background: var(--surface-2);
	}
</style>
