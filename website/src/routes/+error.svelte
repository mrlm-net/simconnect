<script lang="ts">
	import { base } from '$app/paths';
	import Header from '$lib/components/layout/Header.svelte';
	import Footer from '$lib/components/layout/Footer.svelte';
	import { page } from '$app/stores';
	import { siteConfig } from '$lib/config/site.js';
</script>

<svelte:head>
	<title>{$page.status} — {siteConfig.title}</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<Header {siteConfig} onToggleSidebar={() => {}} showMenuButton={false} />
<div class="relative flex flex-1 flex-col overflow-hidden pt-16">

	<!-- Main content -->
	<main class="relative z-10 flex flex-1 flex-col items-center justify-center px-6 py-24 text-center">
		<!-- Status code -->
		<p
			class="mb-4 font-mono text-8xl font-semibold tabular-nums md:text-9xl"
			style="color: var(--brand);"
		>
			{$page.status}
		</p>

		<!-- Heading -->
		<h1 class="mb-3 text-2xl font-semibold tracking-tight md:text-3xl" style="color: var(--color-text-primary);">
			{#if $page.status === 404}
				Waypoint not in navigation database.
			{:else}
				Something went wrong.
			{/if}
		</h1>

		<!-- Description -->
		<p class="mx-auto mb-6 max-w-md text-base leading-relaxed" style="color: var(--color-text-secondary);">
			{#if $page.status === 404}
				This page wasn't on the flight plan. ATC has no record of the requested route — maybe it got cleared direct to a runway that doesn't exist.
			{:else if $page.error?.message}
				{$page.error.message}
			{:else}
				An unexpected error occurred. Try refreshing the page.
			{/if}
		</p>

		<!-- SimConnect-style exception panel (404 only) -->
		{#if $page.status === 404}
		<div
			class="mx-auto mb-10 w-full max-w-sm overflow-hidden rounded-lg border text-left font-mono text-xs"
			style="background-color: var(--color-bg-code); border-color: var(--color-border);"
		>
			<div
				class="flex items-center gap-2 border-b px-4 py-2"
				style="background-color: var(--color-bg-tertiary); border-color: var(--color-border);"
			>
				<span class="inline-block h-2 w-2 rounded-full" style="background-color: var(--danger);"></span>
				<span style="color: var(--color-text-muted);">simconnect exception</span>
			</div>
			<div class="px-4 py-3 leading-relaxed">
				<p style="color: var(--danger);">SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED</p>
				<p class="mt-1" style="color: var(--color-text-muted);">
					exceptionID=2 &nbsp;·&nbsp; sendID=404 &nbsp;·&nbsp; index=0
				</p>
				<p class="mt-2" style="color: var(--color-text-secondary);">
					<span style="color: var(--color-text-muted);">goroutine 1</span> [running]:<br />
					panic: page not found — <span style="color: var(--text);">go get</span> a valid URL
				</p>
			</div>
		</div>
		{:else}
		<div class="mb-10"></div>
		{/if}

		<!-- Actions -->
		<div class="flex flex-col items-center gap-3 sm:flex-row">
			<a href="{base}/" class="btn btn-primary">
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
				</svg>
				Home
			</a>
			<a href="{base}/docs" class="btn">
				Documentation
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
				</svg>
			</a>
		</div>
	</main>
</div>
<Footer {siteConfig} />
