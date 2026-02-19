<script lang="ts">
	import '../app.css';
	import Header from '$lib/components/layout/Header.svelte';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import Footer from '$lib/components/layout/Footer.svelte';
	import type { Snippet } from 'svelte';

	let {
		data,
		children
	}: {
		data: { navigation: import('$lib/types/index.js').NavSection[]; siteConfig: import('$lib/types/index.js').SiteConfig };
		children: Snippet;
	} = $props();

	let sidebarOpen = $state(false);

	function toggleSidebar() {
		sidebarOpen = !sidebarOpen;
	}

	function closeSidebar() {
		sidebarOpen = false;
	}
</script>

<div class="min-h-screen flex flex-col">
	<a href="#main-content" class="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:px-4 focus:py-2 focus:bg-[#58a6ff] focus:text-[#0d1117] focus:rounded focus:font-medium">Skip to content</a>
	<Header siteConfig={data.siteConfig} onToggleSidebar={toggleSidebar} />
	<Sidebar navigation={data.navigation} open={sidebarOpen} onClose={closeSidebar} />

	<div class="flex flex-1 pt-16 md:pl-70">
		<main id="main-content" class="flex-1 min-w-0">
			{@render children()}
		</main>
	</div>

	<div class="md:pl-70">
		<Footer siteConfig={data.siteConfig} />
	</div>
</div>
