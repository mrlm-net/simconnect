<script lang="ts">
	import { base } from '$app/paths';
	import Prism from 'prismjs';
	import 'prismjs/components/prism-go';
	import SeoHead from '$lib/components/seo/SeoHead.svelte';
	import JsonLd from '$lib/components/seo/JsonLd.svelte';
	import { siteConfig } from '$lib/config/site.js';

	let { data } = $props();

	let copied = $state(false);

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

		<div class="mt-6 flex flex-wrap items-center justify-center gap-2">
			{#if data.release}
				<a href="https://github.com/mrlm-net/simconnect/releases/latest" target="_blank" rel="noopener noreferrer" class="badge">
					<span class="badge-label">current</span><span class="badge-value">{data.release}</span>
				</a>
			{/if}
			<a href="https://pkg.go.dev/github.com/mrlm-net/simconnect" target="_blank" rel="noopener noreferrer" class="badge">
				<span class="badge-label">go</span><span class="badge-value badge-go">reference</span>
			</a>
			{#if data.milestone}
				<a href="https://github.com/mrlm-net/simconnect/milestone/{data.milestone.number}" target="_blank" rel="noopener noreferrer" class="badge">
					<span class="badge-label">upcoming</span><span class="badge-value">{data.milestone.title}</span>
				</a>
			{/if}
		</div>
	</div>
</section>

<!-- Features -->
<section class="pt-6 pb-20 overflow-hidden" aria-labelledby="features-heading">
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

<!-- CTA Banner -->
<section class="relative isolate overflow-hidden" aria-labelledby="cta-heading">
	<div class="px-6 pt-4 pb-16 sm:pb-20 lg:px-8">
		<div class="mx-auto max-w-2xl text-center">
			<h2 id="cta-heading" class="text-4xl font-semibold tracking-tight text-balance sm:text-5xl" style="color: var(--color-text-primary);">Build powerful MSFS&nbsp;add&#8209;ons with&nbsp;Go.</h2>
			<p class="mx-auto mt-6 max-w-xl text-lg/8 text-pretty" style="color: var(--color-text-secondary);">From real&#8209;time telemetry dashboards to&nbsp;AI&nbsp;traffic controllers&nbsp;&mdash; connect directly to&nbsp;the simulator with a&nbsp;typed, zero&#8209;dependency&nbsp;SDK.</p>
		</div>
	</div>
	<svg viewBox="0 0 1024 1024" aria-hidden="true" class="absolute top-1/2 left-1/2 -z-10 size-256 -translate-x-1/2 mask-[radial-gradient(closest-side,white,transparent)]">
		<circle r="512" cx="512" cy="512" fill="url(#cta-gradient)" fill-opacity="0.08" />
		<defs>
			<radialGradient id="cta-gradient">
				<stop stop-color="#58a6ff" />
				<stop offset="1" stop-color="#a5d6ff" />
			</radialGradient>
		</defs>
	</svg>
</section>

<!-- Code Preview -->
<section class="relative py-20 overflow-hidden" aria-labelledby="code-preview-heading">
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

<!-- Ecosystem -->
<section class="relative py-20 overflow-hidden" aria-labelledby="ecosystem-heading">
	<div class="absolute inset-x-0 top-0 h-px" style="background: linear-gradient(90deg, transparent, var(--color-border) 20%, var(--color-border) 80%, transparent);"></div>
	<div class="mx-auto max-w-5xl px-6">
		<h2 id="ecosystem-heading" class="mb-2 text-center text-sm font-semibold uppercase tracking-widest" style="color: var(--color-text-muted);">
			Ecosystem
		</h2>
		<p class="mx-auto mb-10 max-w-lg text-center text-sm" style="color: var(--color-text-secondary);">
			More tools built on the same SDK — for AI-assisted development and terminal workflows.
		</p>

		<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">

			<!-- MCP Server card -->
			<div
				class="ecosystem-card group relative flex flex-col overflow-hidden rounded-xl border p-6 transition-all duration-200"
				style="background-color: var(--color-bg-secondary); border-color: var(--color-border);"
			>
				<!-- Glow -->
				<div class="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
					style="background: radial-gradient(ellipse 80% 60% at 50% 0%, rgba(168,85,247,0.07) 0%, transparent 70%);"></div>

				<div class="relative flex items-start gap-4">
					<!-- Icon -->
					<div class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border" style="background-color: var(--color-bg-tertiary); border-color: var(--color-border);">
						<svg class="h-5 w-5" style="color: #c084fc;" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
							<path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z" />
							<path stroke-linecap="round" stroke-linejoin="round" d="M18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456z" />
						</svg>
					</div>

					<div class="flex-1 min-w-0">
						<div class="mb-1 flex items-center gap-2 flex-wrap">
							<h3 class="text-base font-semibold" style="color: var(--color-text-primary);">SimConnect MCP</h3>
							<span class="rounded-full px-2 py-0.5 font-mono text-[0.65rem] font-medium" style="background-color: rgba(192,132,252,0.1); color: #c084fc; border: 1px solid rgba(192,132,252,0.2);">MCP server</span>
						</div>
						<p class="text-sm leading-relaxed" style="color: var(--color-text-secondary);">
							Let AI assistants query live simulator data and the full SimConnect SDK reference through the Model Context Protocol. Works with Claude Code, Claude Desktop, and any MCP-compatible client.
						</p>
					</div>
				</div>

				<div class="relative mt-5 flex flex-wrap gap-2">
					{#each ['1 800+ SimVars', 'Live data', 'Events', 'MSFS 2020 & 2024', '11 MCP tools'] as tag}
						<span class="rounded-full border px-2.5 py-0.5 text-xs" style="border-color: var(--color-border); color: var(--color-text-muted);">{tag}</span>
					{/each}
				</div>

				<div class="relative mt-auto pt-6 flex items-center justify-between">
					<a
						href="https://simconnect-mcp.mrlm.net/"
						target="_blank"
						rel="noopener noreferrer"
						class="inline-flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm font-semibold transition-all hover:brightness-110"
						style="background-color: rgba(192,132,252,0.12); color: #c084fc; border: 1px solid rgba(192,132,252,0.2);"
					>
						View project
						<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/></svg>
					</a>
					<a
						href="https://github.com/mrlm-net/simconnect-mcp"
						target="_blank"
						rel="noopener noreferrer"
						class="rounded p-1.5 transition-colors"
						style="color: var(--color-text-muted);"
						aria-label="SimConnect MCP on GitHub"
					>
						<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
					</a>
				</div>
			</div>

			<!-- SimVar CLI card -->
			<div
				class="ecosystem-card group relative flex flex-col overflow-hidden rounded-xl border p-6 transition-all duration-200"
				style="background-color: var(--color-bg-secondary); border-color: var(--color-border);"
			>
				<!-- Glow -->
				<div class="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
					style="background: radial-gradient(ellipse 80% 60% at 50% 0%, rgba(63,185,80,0.07) 0%, transparent 70%);"></div>

				<div class="relative flex items-start gap-4">
					<!-- Icon -->
					<div class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border" style="background-color: var(--color-bg-tertiary); border-color: var(--color-border);">
						<svg class="h-5 w-5" style="color: #3fb950;" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
							<path stroke-linecap="round" stroke-linejoin="round" d="M6.75 7.5l3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0021 18V6a2.25 2.25 0 00-2.25-2.25H5.25A2.25 2.25 0 003 6v12a2.25 2.25 0 002.25 2.25z" />
						</svg>
					</div>

					<div class="flex-1 min-w-0">
						<div class="mb-1 flex items-center gap-2 flex-wrap">
							<h3 class="text-base font-semibold" style="color: var(--color-text-primary);">simvar-cli</h3>
							<span class="rounded-full px-2 py-0.5 font-mono text-[0.65rem] font-medium" style="background-color: rgba(63,185,80,0.1); color: #3fb950; border: 1px solid rgba(63,185,80,0.2);">CLI tool</span>
						</div>
						<p class="text-sm leading-relaxed" style="color: var(--color-text-secondary);">
							Read, write, and stream MSFS simulation variables straight from the terminal. REPL, watch mode with CSV/JSON output, and a config file — zero extra installs beyond the Go toolchain.
						</p>
					</div>
				</div>

				<div class="relative mt-5 overflow-hidden rounded-lg border font-mono text-xs" style="background-color: var(--color-bg-code); border-color: var(--color-border);">
					<div class="border-b px-3 py-1.5" style="background-color: var(--color-bg-tertiary); border-color: var(--color-border); color: var(--color-text-muted);">$ simvar-cli</div>
					<div class="space-y-0.5 px-3 py-2.5" style="color: var(--color-text-secondary);">
						<div><span style="color: var(--color-text-muted);">›</span> get <span style="color: #a5d6ff;">"PLANE ALTITUDE"</span> feet float64</div>
						<div><span style="color: var(--color-text-muted);">›</span> watch <span style="color: #a5d6ff;">"AIRSPEED INDICATED"</span> knots float64</div>
						<div><span style="color: var(--color-text-muted);">›</span> set <span style="color: #a5d6ff;">"AUTOPILOT HEADING LOCK DIR"</span> degrees 270</div>
					</div>
				</div>

				<div class="relative mt-auto pt-6 flex items-center justify-between">
					<a
						href="{base}/docs/simvar-cli"
						class="inline-flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm font-semibold transition-all hover:brightness-110"
						style="background-color: rgba(63,185,80,0.12); color: #3fb950; border: 1px solid rgba(63,185,80,0.2);"
					>
						Documentation
						<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6"/></svg>
					</a>
					<a
						href="https://github.com/mrlm-net/simconnect/tree/main/cmd/simvar-cli"
						target="_blank"
						rel="noopener noreferrer"
						class="rounded p-1.5 transition-colors"
						style="color: var(--color-text-muted);"
						aria-label="simvar-cli on GitHub"
					>
						<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
					</a>
				</div>
			</div>

		</div>
	</div>
</section>

<!-- Sponsoring -->
<section class="relative py-20" aria-labelledby="sponsoring-heading">
	<div class="absolute inset-x-0 top-0 h-px" style="background: linear-gradient(90deg, transparent, var(--color-border) 20%, var(--color-border) 80%, transparent);"></div>
	<div
		class="pointer-events-none absolute inset-0"
		style="background: radial-gradient(ellipse 60% 80% at 50% 50%, rgba(88,166,255,0.05) 0%, transparent 70%);"
	></div>
	<div class="relative mx-auto max-w-xl px-6 text-center">
		<h2
			id="sponsoring-heading"
			class="mb-3 text-sm font-semibold uppercase tracking-widest"
			style="color: var(--color-text-muted);"
		>
			Support the project
		</h2>
		<p class="mb-4 text-2xl font-bold tracking-tight" style="color: var(--color-text-primary);">
			Back open-source MSFS tooling
		</p>
		<p class="mx-auto mb-8 max-w-md text-base leading-relaxed text-pretty" style="color: var(--color-text-secondary);">
			Sponsoring covers infrastructure costs, development time, and MSFS&nbsp;2020&nbsp;&amp;&nbsp;2024
			licences required to test against real simulator versions.
		</p>
		<a
			href="https://revolut.me/mrlm?currency=EUR"
			target="_blank"
			rel="noopener noreferrer"
			class="inline-flex items-center gap-2 rounded-lg px-5 py-2.5 text-sm font-semibold shadow-sm transition-colors hover:brightness-110"
			style="background-color: var(--color-link); color: var(--color-bg-primary);"
		>
			Sponsor via Revolut
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
		</a>
	</div>
</section>

<!-- Quick Links -->
<section class="py-20 overflow-hidden" aria-labelledby="quick-links-heading">
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
<section class="py-10 overflow-hidden">
	<div class="mx-auto max-w-4xl px-6 text-center">
		<p class="text-sm" style="color: var(--color-text-muted);">
			Microsoft Flight Simulator&nbsp;2020&nbsp;&&nbsp;2024 &middot; Go&nbsp;1.25+ &middot; Windows &middot; Zero external dependencies
		</p>
	</div>
</section>

<style>
	.badge {
		display: inline-flex;
		font-size: 0.6875rem;
		line-height: 1;
		border-radius: 0.25rem;
		overflow: hidden;
		text-decoration: none;
		font-family: var(--font-mono);
	}

	.badge-label {
		padding: 0.25rem 0.4rem;
		background-color: var(--color-bg-tertiary);
		color: var(--color-text-secondary);
		font-weight: 300;
	}

	.badge-value {
		padding: 0.25rem 0.4rem;
		background-color: var(--color-link);
		color: #fff;
	}

	.badge-go {
		background-color: #007d9c;
		color: #fff;
	}

	a.group:hover {
		border-color: var(--color-link) !important;
		box-shadow: 0 0 12px 2px rgba(88, 166, 255, 0.25);
	}

	.ecosystem-card:hover {
		border-color: var(--color-border-active);
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
