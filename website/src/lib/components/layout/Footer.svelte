<script lang="ts">
	import type { SiteConfig } from '$lib/types/index.js';

	let { siteConfig }: { siteConfig: SiteConfig } = $props();
</script>

<footer
	class="border-t px-6 py-4 text-center text-sm"
	style="background-color: var(--color-bg-primary); border-color: var(--color-border); color: var(--color-text-muted);"
>
	<span>Built with SvelteKit</span>
	<span class="mx-2">|</span>
	<a
		href={siteConfig.repoUrl}
		target="_blank"
		rel="noopener noreferrer"
		class="transition-colors"
		style="color: var(--color-link);"
	>
		GitHub
	</a>
</footer>
