<script lang="ts">
	import '../app.css';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div class="min-h-screen flex flex-col">
	<a
		href="#main-content"
		class="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:px-4 focus:py-2 focus:bg-[#58a6ff] focus:text-[#0d1117] focus:rounded focus:font-medium"
	>
		Skip to content
	</a>
	{@render children()}
</div>
