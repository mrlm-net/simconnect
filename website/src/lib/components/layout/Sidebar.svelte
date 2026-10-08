<script lang="ts">
	import { page } from '$app/state';
	import type { NavSection, NavItem } from '$lib/types/index.js';

	let {
		navigation,
		topLinks = [],
		open,
		onClose
	}: { navigation: NavSection[]; topLinks?: NavItem[]; open: boolean; onClose: () => void } =
		$props();

	let sectionState = $state<Record<string, boolean>>({});

	$effect(() => {
		for (const section of navigation) {
			if (!(section.id in sectionState)) {
				sectionState[section.id] = section.defaultOpen ?? false;
			}
		}
	});

	function toggleSection(id: string) {
		sectionState[id] = !sectionState[id];
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && open) {
			onClose();
		}
	}

	function isActive(href: string): boolean {
		const path = page.url.pathname.replace(/\/$/, '');
		return path === href.replace(/\/$/, '');
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<div class="fixed inset-0 z-40 cursor-pointer bg-black/40 md:hidden" onclick={onClose} role="presentation"></div>
{/if}

<aside
	class="sidebar fixed top-16 bottom-0 left-0 z-50 w-70 overflow-y-auto transition-transform duration-200 ease-in-out md:z-30 md:translate-x-0"
	class:max-md:-translate-x-full={!open}
	class:max-md:translate-x-0={open}
>
	<nav class="px-3 py-5" aria-label="Documentation">
		{#if topLinks && topLinks.length > 0}
			<ul class="mb-4 space-y-0.5">
				{#each topLinks as link (link.href)}
					{@const active = isActive(link.href)}
					<li>
						<a href={link.href} class="item top" class:on={active} aria-current={active ? 'page' : undefined} onclick={onClose}>{link.title}</a>
					</li>
				{/each}
			</ul>
		{/if}

		{#each navigation as section (section.id)}
			<div class="mb-4">
				<button class="section eyebrow" onclick={() => toggleSection(section.id)} aria-expanded={sectionState[section.id] ?? false}>
					{section.title}
					<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="transition-transform duration-150" class:rotate-90={sectionState[section.id]} aria-hidden="true">
						<polyline points="9 18 15 12 9 6" />
					</svg>
				</button>

				{#if sectionState[section.id]}
					<ul class="mt-1 space-y-px">
						{#each section.items as item (item.href)}
							{@const active = isActive(item.href)}
							<li>
								<a href={item.href} class="item" class:on={active} aria-current={active ? 'page' : undefined} onclick={onClose}>{item.title}</a>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		{/each}
	</nav>
</aside>

<style>
	.sidebar {
		background: var(--bg);
		border-right: 1px solid var(--border);
	}
	.section {
		display: flex;
		width: 100%;
		align-items: center;
		justify-content: space-between;
		padding: 0.35rem 0.6rem;
		cursor: pointer;
	}
	.section:hover {
		color: var(--text-2);
	}
	.item {
		display: block;
		padding: 0.38rem 0.6rem;
		border-radius: 6px;
		font-size: 0.875rem;
		line-height: 1.35;
		color: var(--text-2);
		transition: color 120ms, background-color 120ms;
	}
	.item.top {
		font-weight: 500;
		color: var(--text);
	}
	.item:hover {
		color: var(--text);
		background: var(--surface-2);
	}
	.item.on {
		color: var(--text);
		font-weight: 500;
		background: var(--surface);
		box-shadow: inset 2px 0 0 var(--brand), var(--shadow-1);
	}
</style>
