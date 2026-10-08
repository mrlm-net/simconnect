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
	<nav class="toc pt-10 pb-6 pr-6" aria-label="Table of contents">
		<p class="eyebrow mb-3">On this page</p>
		<ul class="text-[0.8125rem]">
			{#each headings as heading (heading.id)}
				{@const active = activeId === heading.id}
				<li>
					<a
						href="#{heading.id}"
						class="entry"
						class:on={active}
						style="padding-left: {heading.depth >= 4 ? 1.75 : heading.depth === 3 ? 1.15 : 0.75}rem;"
					>
						{heading.text}
					</a>
				</li>
			{/each}
		</ul>
	</nav>
{/if}

<style>
	.toc {
		width: 260px;
	}
	.entry {
		display: block;
		padding-top: 0.22rem;
		padding-bottom: 0.22rem;
		border-left: 1px solid var(--border);
		color: var(--text-3);
		line-height: 1.4;
		transition: color 120ms, border-color 120ms;
	}
	.entry:hover {
		color: var(--text);
	}
	.entry.on {
		color: var(--text);
		border-left-color: var(--brand);
		box-shadow: inset 1px 0 0 var(--brand);
	}
</style>
