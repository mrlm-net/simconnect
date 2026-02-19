<script lang="ts">
	import type { TocEntry } from '$lib/types/index.js';

	let { headings }: { headings: TocEntry[] } = $props();

	let activeId = $state('');

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
</script>

{#if headings.length > 0}
	<div
		class="sticky top-20 hidden max-h-[calc(100vh-6rem)] overflow-y-auto xl:block"
		style="width: 240px;"
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
						href="#{heading.id}"
						class="block py-0.5 transition-colors"
						style="color: {active ? 'var(--color-link)' : 'var(--color-text-muted)'};"
					>
						{heading.text}
					</a>
				</li>
			{/each}
		</ul>
	</div>
{/if}
