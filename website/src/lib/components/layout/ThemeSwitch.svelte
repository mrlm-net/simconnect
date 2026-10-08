<script lang="ts">
	type Choice = 'light' | 'dark' | 'system';

	let choice = $state<Choice>('system');

	$effect(() => {
		const t = document.documentElement.dataset.theme;
		choice = t === 'light' || t === 'dark' ? t : 'system';
	});

	function pick(next: Choice) {
		choice = next;
		const root = document.documentElement;
		try {
			if (next === 'system') {
				delete root.dataset.theme;
				localStorage.removeItem('theme');
			} else {
				root.dataset.theme = next;
				localStorage.setItem('theme', next);
			}
		} catch {
			// storage blocked: the choice still applies to this page
		}
	}

	const options: { id: Choice; label: string }[] = [
		{ id: 'light', label: 'Light theme' },
		{ id: 'dark', label: 'Dark theme' },
		{ id: 'system', label: 'System theme' }
	];
</script>

<div class="theme-switch" role="group" aria-label="Theme">
	{#each options as o (o.id)}
		<button type="button" aria-pressed={choice === o.id} aria-label={o.label} title={o.label} onclick={() => pick(o.id)}>
			<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				{#if o.id === 'light'}
					<circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
				{:else if o.id === 'dark'}
					<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
				{:else}
					<rect x="3" y="4" width="18" height="12" rx="2" /><path d="M8 20h8M12 16v4" />
				{/if}
			</svg>
		</button>
	{/each}
</div>
