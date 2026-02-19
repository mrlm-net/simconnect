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
		return page.url.pathname === href;
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- Backdrop for mobile -->
	<div
		class="fixed inset-0 z-40 bg-black/50 md:hidden"
		onclick={onClose}
		role="presentation"
	></div>
{/if}

<aside
	class="fixed top-16 bottom-0 left-0 z-50 w-70 overflow-y-auto border-r transition-transform duration-200 ease-in-out md:z-30 md:translate-x-0"
	class:max-md:-translate-x-full={!open}
	class:max-md:translate-x-0={open}
	style="background-color: var(--color-bg-secondary); border-color: var(--color-border);"
>
	<nav class="p-4" aria-label="Documentation">
		{#if topLinks && topLinks.length > 0}
			<ul class="mb-4 space-y-0.5">
				{#each topLinks as link}
					{@const active = isActive(link.href)}
					<li>
						<a
							href={link.href}
							class="block rounded-md px-2 py-2 text-sm font-medium transition-colors"
							style="color: {active
								? 'var(--color-link)'
								: 'var(--color-text-primary)'}; {active
								? 'background-color: var(--color-bg-tertiary);'
								: ''}"
							aria-current={active ? 'page' : undefined}
							onclick={onClose}
						>
							{link.title}
						</a>
					</li>
				{/each}
			</ul>
			<div class="mb-3 border-b" style="border-color: var(--color-border);"></div>
		{/if}

		{#each navigation as section}
			<div class="mb-3">
				<button
					class="flex w-full items-center justify-between px-2 py-1.5 text-xs font-semibold uppercase tracking-wider"
					style="color: var(--color-text-muted);"
					onclick={() => toggleSection(section.id)}
					aria-expanded={sectionState[section.id] ?? false}
				>
					{section.title}
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="14"
						height="14"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						class="transition-transform duration-150"
						class:rotate-90={sectionState[section.id]}
					>
						<polyline points="9 18 15 12 9 6" />
					</svg>
				</button>

				{#if sectionState[section.id]}
					<ul class="mt-1 space-y-0.5">
						{#each section.items as item}
							{@const active = isActive(item.href)}
							<li>
								<a
									href={item.href}
									class="block rounded-md px-2 py-1.5 text-sm transition-colors"
									class:font-medium={active}
									style="color: {active
										? 'var(--color-link)'
										: 'var(--color-text-secondary)'}; {active
										? 'background-color: var(--color-bg-tertiary); border-left: 2px solid var(--color-border-active);'
										: 'border-left: 2px solid transparent;'}"
									aria-current={active ? 'page' : undefined}
									onclick={onClose}
								>
									{item.title}
								</a>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		{/each}
	</nav>
</aside>
