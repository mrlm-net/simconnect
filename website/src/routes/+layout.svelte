<script lang="ts">
	import '../app.css';
	import { afterNavigate } from '$app/navigation';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	let isFirstNav = true;
	afterNavigate(() => {
		if (isFirstNav) {
			isFirstNav = false;
			return;
		}
		const main = document.getElementById('main-content');
		if (main) {
			main.focus({ preventScroll: true });
		}
	});
</script>

<div class="min-h-screen flex flex-col">
	{@render children()}
</div>
