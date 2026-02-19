<script lang="ts">
	import type { TocEntry } from '$lib/types/index.js';

	let { headings }: { headings: TocEntry[] } = $props();

	let activeId = $state('');
	let anchorRefs: Record<string, HTMLAnchorElement> = $state({});

	$effect(() => {
		if (typeof IntersectionObserver === 'undefined') return;

		const elements = headings
			.map((h) => document.getElementById(h.id))
			.filter((el): el is HTMLElement => el !== null);

		if (elements.length === 0) return;

		const observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) {
						activeId = entry.target.id;
					}
				}
			},
			{ rootMargin: '-80px 0px -80% 0px', threshold: 0 }
		);

		for (const el of elements) {
			observer.observe(el);
		}

		return () => observer.disconnect();
	});

	$effect(() => {
		if (activeId && anchorRefs[activeId]) {
			anchorRefs[activeId].scrollIntoView({ behavior: 'smooth', block: 'nearest' });
		}
	});
</script>

{#if headings.length > 0}
	<nav
		class="py-6 pl-6 pr-6"
		style="min-width: 280px;"
		aria-label="Table of contents"
	>
		<p
			class="mb-3 text-xs font-semibold uppercase tracking-wider"
			style="color: var(--color-text-muted);"
		>
			On this page
		</p>
		<ul class="space-y-1 text-sm">
			{#each headings as heading}
				{@const active = activeId === heading.id}
				<li style="padding-left: {(heading.depth - 2) * 0.75}rem;">
					<a
						bind:this={anchorRefs[heading.id]}
						href="#{heading.id}"
						class="block py-0.5 transition-colors"
						style="color: {active ? 'var(--color-link)' : 'var(--color-text-muted)'};"
					>
						{heading.text}
					</a>
				</li>
			{/each}
		</ul>
	</nav>
{/if}

<style>
	nav {
		overflow-y: auto;
		scrollbar-width: none;
	}

	nav::-webkit-scrollbar {
		display: none;
	}
</style>
