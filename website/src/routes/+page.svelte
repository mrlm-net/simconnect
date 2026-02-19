<script lang="ts">
	import { base } from '$app/paths';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	onMount(() => {
		goto(`${base}/docs`, { replaceState: true });
	});
</script>

<div class="flex items-center justify-center p-12" style="min-height: calc(100vh - 8rem);">
	<div class="text-center">
		<h1 class="mb-4 text-3xl font-bold" style="color: var(--color-text-primary);">
			SimConnect Go SDK
		</h1>
		<p class="mb-6" style="color: var(--color-text-secondary);">
			GoLang wrapper over SimConnect.dll for MSFS 2020/2024
		</p>
		<a
			href="{base}/docs"
			class="inline-block rounded-lg px-6 py-3 font-medium transition-colors"
			style="background-color: var(--color-link); color: var(--color-bg-primary);"
		>
			View Documentation
		</a>
	</div>
</div>
