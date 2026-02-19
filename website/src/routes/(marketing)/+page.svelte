<script lang="ts">
	import { base } from '$app/paths';
	import Prism from 'prismjs';
	import 'prismjs/components/prism-go';
	import SeoHead from '$lib/components/seo/SeoHead.svelte';
	import JsonLd from '$lib/components/seo/JsonLd.svelte';
	import { siteConfig } from '$lib/config/site.js';

	let copied = $state(false);

	// Dynamic milestone badge
	let milestoneNumber = $state<number | null>(null);
	let milestoneTitle = $state<string | null>(null);
	let milestoneLoaded = $state(false);

	interface Milestone {
		number: number;
		title: string;
		due_on: string | null;
		open_issues: number;
		closed_issues: number;
	}

	$effect(() => {
		fetch('https://api.github.com/repos/mrlm-net/simconnect/milestones?state=open&sort=due_on&direction=asc')
			.then((res) => {
				if (!res.ok) throw new Error(`HTTP ${res.status}`);
				return res.json();
			})
			.then((milestones: Milestone[]) => {
				const candidates = milestones
					.filter((m) => m.due_on !== null && (m.open_issues + m.closed_issues) > 0)
					.sort((a, b) => a.title.localeCompare(b.title, undefined, { numeric: true }));
				if (candidates.length === 0) return;
				milestoneNumber = candidates[0].number;
				milestoneTitle = candidates[0].title;
			})
			.catch((err) => console.warn('[milestone badge]', err))
			.finally(() => {
				milestoneLoaded = true;
			});
	});

	function copyInstall() {
		navigator.clipboard.writeText('go get github.com/mrlm-net/simconnect');
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	const clientCode = `package main

import (
    "log"
    "github.com/mrlm-net/simconnect"
)

func main() {
    client := simconnect.NewClient("My App")

    if err := client.Connect(); err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect()

    // Read simulator data, emit events, etc.
}`;

	const managerCode = `package main

import (
    "fmt"
    "github.com/mrlm-net/simconnect/pkg/manager"
)

func main() {
    mgr := manager.New("My App",
        manager.WithAutoReconnect(true),
    )

    mgr.OnConnectionStateChange(
        func(old, current manager.ConnectionState) {
            fmt.Printf("%s -> %s\\n", old, current)
        },
    )

    mgr.Start()
}`;

	const clientHighlighted = Prism.highlight(clientCode, Prism.languages['go'], 'go');
	const managerHighlighted = Prism.highlight(managerCode, Prism.languages['go'], 'go');

	const features = [
		{
			title: 'Zero Dependencies',
			description: 'Standard library only. No external packages to manage, audit, or update.',
			icon: 'cube'
		},
		{
			title: 'Fully Typed API',
			description: 'Strong typing with dedicated structs, enums, and typed constants throughout.',
			icon: 'shield'
		},
		{
			title: 'Auto-Reconnect',
			description:
				'Built-in connection lifecycle management with automatic reconnection support.',
			icon: 'refresh'
		},
		{
			title: 'Ready-Made Datasets',
			description:
				'Pre-built dataset definitions for aircraft, environment, facilities, and traffic.',
			icon: 'layers'
		},
		{
			title: 'Tiered Buffer Pooling',
			description: 'Optimized 4KB/16KB/64KB buffer pools for zero-allocation hot paths.',
			icon: 'bolt'
		},
		{
			title: 'MSFS\u00a02020\u00a0&\u00a02024',
			description: 'Full compatibility with both simulator generations out of the box.',
			icon: 'plane'
		}
	];

	const quickLinks = [
		{
			title: 'Getting Started',
			description: 'Install, connect, and read your first SimVar in minutes.',
			href: `${base}/getting-started`
		},
		{
			title: 'Client API',
			description: 'Direct SimConnect communication for maximum control.',
			href: `${base}/docs/usage-client`
		},
		{
			title: 'Manager API',
			description: 'Auto-reconnect, state tracking, and lifecycle events.',
			href: `${base}/docs/usage-manager`
		}
	];
</script>

<SeoHead
	{siteConfig}
	title="SimConnect Go SDK — GoLang wrapper for MSFS 2020/2024"
	description="Build Microsoft Flight Simulator add-ons with Go. Lightweight, typed, zero-dependency SimConnect wrapper."
	path="/"
/>
<JsonLd
	schema={{
		'@context': 'https://schema.org',
		'@type': 'WebSite',
		name: siteConfig.title,
		url: `${siteConfig.url}${siteConfig.basePath}/`,
		description: siteConfig.description
	}}
/>
<JsonLd
	schema={{
		'@context': 'https://schema.org',
		'@type': 'SoftwareSourceCode',
		name: siteConfig.title,
		description: siteConfig.description,
		codeRepository: siteConfig.repoUrl,
		programmingLanguage: 'Go',
		runtimePlatform: 'Windows',
		license: `https://opensource.org/licenses/${siteConfig.license}`
	}}
/>

<!-- Hero -->
<section class="relative overflow-hidden py-24 md:py-36">
	<!-- Subtle grid background -->
	<div
		class="pointer-events-none absolute inset-0 opacity-[0.03]"
		style="background-image: linear-gradient(var(--color-text-muted) 1px, transparent 1px), linear-gradient(90deg, var(--color-text-muted) 1px, transparent 1px); background-size: 64px 64px;"
	></div>
	<!-- Top-down radial fade -->
	<div
		class="pointer-events-none absolute inset-0"
		style="background: radial-gradient(ellipse 60% 50% at 50% 0%, rgba(88,166,255,0.08) 0%, transparent 70%);"
	></div>

	<div class="relative mx-auto max-w-4xl px-6 text-center">
		<div
			class="mb-6 inline-flex items-center gap-2 rounded-full border px-4 py-1.5 text-xs font-medium tracking-wide"
			style="border-color: var(--color-border); color: var(--color-text-muted); background-color: var(--color-bg-secondary);"
		>
			<span class="inline-block h-1.5 w-1.5 rounded-full" style="background-color: #3fb950;"></span>
			Go 1.25+ &middot; Windows &middot; SimConnect.dll
		</div>

		<h1
			class="mb-6 text-5xl font-bold tracking-tight md:text-6xl lg:text-7xl"
			style="color: var(--color-text-primary);"
		>
			SimConnect
			<span
				style="background: linear-gradient(to right, #58a6ff, #a5d6ff); -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text;"
			>
				Go SDK
			</span>
		</h1>

		<p class="mx-auto mb-10 max-w-2xl text-lg leading-relaxed md:text-xl" style="color: var(--color-text-secondary);">
			Build Microsoft Flight Simulator add-ons with Go.
			<span style="color: var(--color-text-primary);">Lightweight, typed, zero-dependency</span>
			wrapper over SimConnect.dll for&nbsp;MSFS&nbsp;2020&nbsp;&&nbsp;2024.
		</p>

		<div class="mb-10 flex flex-col items-center justify-center gap-4 sm:flex-row">
			<a
				href="{base}/getting-started"
				class="group inline-flex items-center gap-2 rounded-lg px-7 py-3.5 text-base font-semibold transition-all hover:brightness-110"
				style="background-color: var(--color-link); color: var(--color-bg-primary);"
			>
				Get Started
				<svg class="h-4 w-4 transition-transform group-hover:translate-x-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" /></svg>
			</a>
			<a
				href="{base}/docs"
				class="inline-flex items-center rounded-lg border px-7 py-3.5 text-base font-semibold transition-colors hover:border-[var(--color-text-muted)]"
				style="border-color: var(--color-border); color: var(--color-text-primary);"
			>
				Documentation
			</a>
		</div>

		<div class="relative mx-auto inline-block">
			<div
				class="inline-flex items-center gap-3 rounded-lg border px-5 py-3 font-mono text-sm"
				style="background-color: var(--color-bg-code); border-color: var(--color-border);"
			>
				<span style="color: var(--color-text-muted);">$</span>
				<span><span style="color: var(--color-text-secondary);">go get</span> <span style="color: var(--color-link);">github.com/mrlm-net/simconnect</span></span>
				<button
					class="cursor-pointer rounded p-1 transition-colors hover:bg-white/5"
					style="color: {copied ? '#3fb950' : 'var(--color-text-muted)'};"
					aria-label="Copy install command"
					onclick={copyInstall}
				>
					{#if copied}
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" /></svg>
					{:else}
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
					{/if}
				</button>
			</div>
		</div>

		<div class="mt-6 flex flex-wrap items-center justify-center gap-3">
			<a href="https://github.com/mrlm-net/simconnect/releases/latest" target="_blank" rel="noopener noreferrer">
				<img src="https://img.shields.io/github/v/release/mrlm-net/simconnect?label=release" alt="Latest Release" height="20" class="h-5" />
			</a>
			<a href="https://pkg.go.dev/github.com/mrlm-net/simconnect" target="_blank" rel="noopener noreferrer">
				<img src="https://img.shields.io/badge/go-reference-007d9c?logo=go&logoColor=white" alt="Go Reference" height="20" class="h-5" />
			</a>
			{#if milestoneNumber && milestoneTitle}
				<a href="https://github.com/mrlm-net/simconnect/milestone/{milestoneNumber}" target="_blank" rel="noopener noreferrer">
					<img
						src="https://img.shields.io/github/milestones/progress-percent/mrlm-net/simconnect/{milestoneNumber}?label={encodeURIComponent(milestoneTitle ?? '')}"
						alt="{milestoneTitle} Progress"
						height="20"
						class="h-5"
					/>
				</a>
			{:else if !milestoneLoaded}
				<span class="inline-block h-5 w-20 animate-pulse rounded" style="background-color: var(--color-border);"></span>
			{/if}
		</div>
	</div>
</section>

<!-- Features -->
<section class="py-20" aria-labelledby="features-heading">
	<div class="mx-auto max-w-5xl px-6">
		<h2 id="features-heading" class="mb-12 text-center text-sm font-semibold uppercase tracking-widest" style="color: var(--color-text-muted);">
			Why SimConnect Go SDK
		</h2>
		<div class="grid grid-cols-1 gap-px overflow-hidden rounded-xl border sm:grid-cols-2 lg:grid-cols-3" style="border-color: var(--color-border); background-color: var(--color-border);">
			{#each features as feature}
				<div class="flex flex-col gap-2 p-6" style="background-color: var(--color-bg-secondary);">
					<div class="flex items-center gap-3">
						{#if feature.icon === 'cube'}
							<svg class="h-5 w-5 shrink-0" style="color: var(--color-link);" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" /></svg>
						{:else if feature.icon === 'shield'}
							<svg class="h-5 w-5 shrink-0" style="color: var(--color-link);" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0 1 12 2.944a11.955 11.955 0 0 1-8.618 3.04A12.02 12.02 0 0 0 3 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
						{:else if feature.icon === 'refresh'}
							<svg class="h-5 w-5 shrink-0" style="color: var(--color-link);" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 0 0 4.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 0 1-15.357-2m15.357 2H15" /></svg>
						{:else if feature.icon === 'layers'}
							<svg class="h-5 w-5 shrink-0" style="color: var(--color-link);" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" /></svg>
						{:else if feature.icon === 'bolt'}
							<svg class="h-5 w-5 shrink-0" style="color: var(--color-link);" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
						{:else if feature.icon === 'plane'}
							<svg class="h-5 w-5 shrink-0" style="color: var(--color-link);" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" /></svg>
						{/if}
						<h3 class="text-sm font-semibold" style="color: var(--color-text-primary);">{feature.title}</h3>
					</div>
					<p class="text-sm leading-relaxed" style="color: var(--color-text-secondary);">
						{feature.description}
					</p>
				</div>
			{/each}
		</div>
	</div>
</section>

<!-- Code Preview -->
<section class="relative py-20" aria-labelledby="code-preview-heading">
	<div class="absolute inset-x-0 top-0 h-px" style="background: linear-gradient(90deg, transparent, var(--color-border) 20%, var(--color-border) 80%, transparent);"></div>
	<div class="mx-auto max-w-5xl px-6">
		<h2 id="code-preview-heading" class="mb-2 text-center text-sm font-semibold uppercase tracking-widest" style="color: var(--color-text-muted);">
			Two ways to connect
		</h2>
		<p class="mx-auto mb-10 max-w-lg text-center text-sm" style="color: var(--color-text-secondary);">
			Use the low-level client for full control, or the manager for production-ready lifecycle management.
		</p>
		<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
			<div
				class="overflow-hidden rounded-xl border transition-colors"
				style="background-color: var(--color-bg-code); border-color: var(--color-border);"
			>
				<div
					class="flex items-center gap-2 border-b px-4 py-2.5"
					style="background-color: var(--color-bg-tertiary); border-color: var(--color-border);"
				>
					<span class="dot-pulse inline-block h-2.5 w-2.5 rounded-full" style="background-color: var(--color-link);"></span>
					<span class="font-mono text-xs" style="color: var(--color-text-muted);">Low-Level Client</span>
				</div>
				<pre class="overflow-x-auto p-4"><code class="language-go text-[0.8125rem] leading-relaxed">{@html clientHighlighted}</code></pre>
			</div>

			<div
				class="overflow-hidden rounded-xl border transition-colors"
				style="background-color: var(--color-bg-code); border-color: var(--color-border);"
			>
				<div
					class="flex items-center gap-2 border-b px-4 py-2.5"
					style="background-color: var(--color-bg-tertiary); border-color: var(--color-border);"
				>
					<span class="dot-pulse inline-block h-2.5 w-2.5 rounded-full" style="background-color: #3fb950;"></span>
					<span class="font-mono text-xs" style="color: var(--color-text-muted);">Manager with Auto-Reconnect</span>
				</div>
				<pre class="overflow-x-auto p-4"><code class="language-go text-[0.8125rem] leading-relaxed">{@html managerHighlighted}</code></pre>
			</div>
		</div>
	</div>
</section>

<!-- Quick Links -->
<section class="py-20" aria-labelledby="quick-links-heading">
	<div class="mx-auto max-w-4xl px-6">
		<h2 id="quick-links-heading" class="mb-10 text-center text-sm font-semibold uppercase tracking-widest" style="color: var(--color-text-muted);">
			Explore the docs
		</h2>
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			{#each quickLinks as link}
				<a
					href={link.href}
					class="group relative overflow-hidden rounded-xl border p-6 transition-all duration-200"
					style="background-color: var(--color-bg-secondary); border-color: var(--color-border);"
				>
					<h3 class="mb-2 flex items-center gap-2 text-base font-semibold" style="color: var(--color-text-primary);">
						{link.title}
						<svg
							class="h-4 w-4 transition-transform group-hover:translate-x-1"
							style="color: var(--color-text-muted);"
							fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
						><path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" /></svg>
					</h3>
					<p class="text-sm leading-relaxed" style="color: var(--color-text-secondary);">
						{link.description}
					</p>
				</a>
			{/each}
		</div>
	</div>
</section>

<!-- Compatibility -->
<section class="py-10">
	<div class="mx-auto max-w-4xl px-6 text-center">
		<p class="text-sm" style="color: var(--color-text-muted);">
			Microsoft Flight Simulator&nbsp;2020&nbsp;&&nbsp;2024 &middot; Go&nbsp;1.25+ &middot; Windows &middot; Zero external dependencies
		</p>
	</div>
</section>

<style>
	a.group:hover {
		border-color: var(--color-link) !important;
		box-shadow: 0 0 12px 2px rgba(88, 166, 255, 0.25);
	}

	@keyframes dot-pulse {
		0%, 100% {
			opacity: 0.4;
		}
		50% {
			opacity: 1;
		}
	}

	.dot-pulse {
		animation: dot-pulse 3s ease-in-out infinite;
	}

	@media (prefers-reduced-motion: reduce) {
		.dot-pulse {
			animation: none;
			opacity: 0.7;
		}
	}
</style>
