<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import Footer from '$lib/components/layout/Footer.svelte';
	import type { Snippet } from 'svelte';
	import type { NavSection, NavItem, SiteConfig } from '$lib/types/index.js';

	let {
		data,
		children
	}: {
		data: { navigation: NavSection[]; topLinks: NavItem[]; siteConfig: SiteConfig };
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

<Header siteConfig={data.siteConfig} onToggleSidebar={toggleSidebar} />
<Sidebar
	navigation={data.navigation}
	topLinks={data.topLinks}
	open={sidebarOpen}
	onClose={closeSidebar}
/>

<div class="flex flex-1 pt-16 md:pl-70">
	<main id="main-content" class="flex-1 min-w-0">
		{@render children()}
	</main>
</div>

<div class="md:pl-70">
	<Footer siteConfig={data.siteConfig} />
</div>
