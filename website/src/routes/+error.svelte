<script lang="ts">
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import { siteConfig } from '$lib/config/site.js';
</script>

<svelte:head>
	<title>{$page.status} — {siteConfig.title}</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<div class="relative flex min-h-screen flex-col overflow-hidden" style="background-color: var(--color-bg-primary);">
	<!-- Grid background -->
	<div
		class="pointer-events-none absolute inset-0 opacity-[0.03]"
		style="background-image: linear-gradient(var(--color-text-muted) 1px, transparent 1px), linear-gradient(90deg, var(--color-text-muted) 1px, transparent 1px); background-size: 64px 64px;"
	></div>
	<!-- Radial glow -->
	<div
		class="pointer-events-none absolute inset-0"
		style="background: radial-gradient(ellipse 60% 50% at 50% 30%, rgba(88,166,255,0.06) 0%, transparent 70%);"
	></div>

	<!-- Minimal header -->
	<header
		class="relative z-10 flex h-16 items-center justify-between border-b px-6"
		style="background-color: var(--color-bg-secondary); border-color: var(--color-border);"
	>
		<a
			href="{base}/"
			class="flex items-center gap-2 text-lg font-semibold tracking-tight"
			style="color: var(--color-text-primary);"
		>
			<img src="{base}/icon.png" alt="" class="h-8 w-auto" />
			<span class="hidden md:inline">{siteConfig.title}</span>
		</a>
		<a
			href={siteConfig.repoUrl}
			target="_blank"
			rel="noopener noreferrer"
			class="rounded p-1.5 transition-colors"
			style="color: var(--color-text-secondary);"
			aria-label="View on GitHub"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
				<path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
			</svg>
		</a>
	</header>

	<!-- Main content -->
	<main class="relative z-10 flex flex-1 flex-col items-center justify-center px-6 py-24 text-center">
		<!-- Status code -->
		<p
			class="mb-4 font-mono text-8xl font-bold tabular-nums md:text-9xl"
			style="background: linear-gradient(to right, #58a6ff, #a5d6ff); -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text;"
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
				<span class="inline-block h-2 w-2 rounded-full" style="background-color: #f85149;"></span>
				<span style="color: var(--color-text-muted);">simconnect exception</span>
			</div>
			<div class="px-4 py-3 leading-relaxed">
				<p style="color: #f85149;">SIMCONNECT_EXCEPTION_NAME_UNRECOGNIZED</p>
				<p class="mt-1" style="color: var(--color-text-muted);">
					exceptionID=2 &nbsp;·&nbsp; sendID=404 &nbsp;·&nbsp; index=0
				</p>
				<p class="mt-2" style="color: var(--color-text-secondary);">
					<span style="color: var(--color-text-muted);">goroutine 1</span> [running]:<br />
					panic: page not found — <span style="color: var(--color-link);">go get</span> a valid URL
				</p>
			</div>
		</div>
		{:else}
		<div class="mb-10"></div>
		{/if}

		<!-- Actions -->
		<div class="flex flex-col items-center gap-3 sm:flex-row">
			<a
				href="{base}/"
				class="inline-flex items-center gap-2 rounded-lg px-6 py-3 text-sm font-semibold transition-all hover:brightness-110"
				style="background-color: var(--color-link); color: var(--color-bg-primary);"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
				</svg>
				Home
			</a>
			<a
				href="{base}/docs"
				class="inline-flex items-center gap-2 rounded-lg border px-6 py-3 text-sm font-semibold transition-colors hover:border-[var(--color-text-muted)]"
				style="border-color: var(--color-border); color: var(--color-text-primary);"
			>
				Documentation
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
				</svg>
			</a>
		</div>
	</main>
</div>
