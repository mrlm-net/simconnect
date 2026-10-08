<script lang="ts">
	import { base } from '$app/paths';
	import Prism from 'prismjs';
	import 'prismjs/components/prism-go';
	import SeoHead from '$lib/components/seo/SeoHead.svelte';
	import JsonLd from '$lib/components/seo/JsonLd.svelte';
	import { siteConfig } from '$lib/config/site.js';

	let { data } = $props();

	let copied = $state('');

	const installCommand = 'go get github.com/mrlm-net/simconnect';
	// From the repository root: the map is its own module, -C runs it from its folder.
	const runMap = 'go run -C cmd/airport-map .';

	async function copy(text: string) {
		try {
			await navigator.clipboard.writeText(text);
			copied = text;
			setTimeout(() => {
				if (copied === text) copied = '';
			}, 2000);
		} catch {
			// clipboard not available
		}
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

	const samples = [
		{ label: 'Low-level client', html: Prism.highlight(clientCode, Prism.languages['go'], 'go') },
		{ label: 'Manager with auto-reconnect', html: Prism.highlight(managerCode, Prism.languages['go'], 'go') }
	];

	// Icon paths: 24×24, stroke 1.75
	const features = [
		{
			title: 'Zero Dependencies',
			description: 'Standard library only. No external packages to manage, audit, or update.',
			icon: '<path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>'
		},
		{
			title: 'Fully Typed API',
			description: 'Strong typing with dedicated structs, enums, and typed constants throughout.',
			icon: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M9 12l2 2 4-4"/>'
		},
		{
			title: 'Auto-Reconnect',
			description: 'Built-in connection lifecycle management with automatic reconnection support.',
			icon: '<path d="M21 12a9 9 0 0 1-15.5 6.2L3 16M3 12a9 9 0 0 1 15.5-6.2L21 8"/><path d="M21 3v5h-5M3 21v-5h5"/>'
		},
		{
			title: 'Ready-Made Datasets',
			description: 'Pre-built dataset definitions for aircraft, environment, facilities, and traffic.',
			icon: '<path d="M12 2 2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>'
		},
		{
			title: 'Tiered Buffer Pooling',
			description: 'Optimized 4KB/16KB/64KB buffer pools for zero-allocation hot paths.',
			icon: '<path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z"/>'
		},
		{
			title: 'MSFS 2020 & 2024',
			description: 'Full compatibility with both simulator generations out of the box.',
			icon: '<path d="M17.8 19.2 16 11l3.5-3.5C21 6 21.5 4 21 3c-1-.5-3 0-4.5 1.5L13 8 4.8 6.2c-.5-.1-.9.1-1.1.5l-.3.5c-.2.5-.1 1 .3 1.3L9 12l-2 3H4l-1 1 3 2 2 3 1-1v-3l3-2 3.5 5.3c.3.4.8.5 1.3.3l.5-.2c.4-.3.6-.7.5-1.2z"/>'
		}
	];

	const quickLinks = [
		{ title: 'Getting Started', description: 'Install, connect, and read your first SimVar in minutes.', href: `${base}/getting-started` },
		{ title: 'Airport Map', description: 'The main example and debugging tool: layout, traffic, approach and tower.', href: `${base}/docs/examples` },
		{ title: 'Client API', description: 'Direct SimConnect communication for maximum control.', href: `${base}/docs/usage-client` },
		{ title: 'Manager API', description: 'Auto-reconnect, state tracking, and lifecycle events.', href: `${base}/docs/usage-manager` }
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
		license: `${siteConfig.repoUrl}/blob/main/LICENSE`
	}}
/>

{#snippet arrow()}
	<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12h14M12 5l7 7-7 7" /></svg>
{/snippet}

{#snippet external()}
	<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6M15 3h6v6M10 14 21 3" /></svg>
{/snippet}

{#snippet command(text: string, label: string)}
	<div class="cmd">
		<code><span style="color: var(--text-3);">$&nbsp;</span>{text}</code>
		<button onclick={() => copy(text)} aria-label={label} title="Copy to clipboard" class:ok={copied === text}>
			<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				{#if copied === text}
					<polyline points="20 6 9 17 4 12" />
				{:else}
					<rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
				{/if}
			</svg>
		</button>
	</div>
{/snippet}

<!-- Hero -->
<section class="hero px-6 pt-16 pb-16 sm:pt-24 sm:pb-20">
	<div class="mx-auto grid max-w-6xl items-center gap-12 lg:grid-cols-[1.1fr_1fr] [&>*]:min-w-0">
		<div>
			<div class="mb-6 flex flex-wrap gap-2">
				<span class="pill"><span class="dot"></span>Go 1.27+ · Windows · SimConnect.dll</span>
			</div>
			<h1 class="mb-5 text-4xl font-semibold tracking-tight sm:text-6xl" style="color: var(--text);">
				SimConnect <span class="brand-word">Go SDK</span>
			</h1>
			<p class="mb-8 max-w-xl text-base leading-relaxed sm:text-lg" style="color: var(--text-2);">
				Build Microsoft Flight Simulator add-ons with Go.
				<span style="color: var(--text);">Lightweight, typed, zero-dependency</span>
				wrapper over SimConnect.dll for&nbsp;MSFS&nbsp;2020&nbsp;&amp;&nbsp;2024.
			</p>
			<div class="mb-8 flex flex-wrap gap-3">
				<a href="{base}/getting-started" class="btn btn-primary">Get started {@render arrow()}</a>
				<a href="{base}/docs" class="btn">Documentation</a>
			</div>
			<div class="max-w-xl">{@render command(installCommand, 'Copy install command')}</div>
			<div class="mt-4 flex flex-wrap gap-2">
				{#if data.release}
					<a href="{siteConfig.repoUrl}/releases/latest" target="_blank" rel="noopener noreferrer" class="pill"><span class="mono">{data.release}</span> current</a>
				{/if}
				<a href="https://pkg.go.dev/github.com/mrlm-net/simconnect" target="_blank" rel="noopener noreferrer" class="pill">Go reference</a>
				{#if data.milestone}
					<a href="{siteConfig.repoUrl}/milestone/{data.milestone.number}" target="_blank" rel="noopener noreferrer" class="pill">upcoming <span class="mono">{data.milestone.title}</span></a>
				{/if}
				<a href="{siteConfig.repoUrl}/blob/main/LICENSE" target="_blank" rel="noopener noreferrer" class="pill">{siteConfig.licenseLabel}</a>
			</div>
		</div>

		<div class="card overflow-hidden">
			<div class="flex items-center justify-between px-4 py-2.5" style="border-bottom: 1px solid var(--border); background: var(--surface-2);">
				<span class="eyebrow">{samples[0].label}</span>
				<span class="eyebrow">main.go</span>
			</div>
			<pre class="code"><code class="language-go">{@html samples[0].html}</code></pre>
		</div>
	</div>
</section>

<!-- The airport map: the main example -->
<section class="px-6 py-20" style="border-top: 1px solid var(--border);" aria-labelledby="map-heading">
	<div class="mx-auto max-w-6xl">
		<p class="eyebrow mb-3">See it running</p>
		<h2 id="map-heading" class="mb-3 text-2xl font-semibold tracking-tight sm:text-3xl" style="color: var(--text);">The airport map</h2>
		<p class="mb-8 max-w-2xl text-sm leading-relaxed sm:text-base" style="color: var(--text-2);">
			The airport map is built only on the SDK: an airport's ground layout as SimConnect reports it, taxi routing, AI traffic under your control, scheduled airlines, the landing sequence and the tower.
		</p>
		<a href="{base}/docs/examples" class="card block overflow-hidden">
			<img
				src="{base}/docs/images/airport-map/ui-traffic.png"
				alt="The airport map at LKPR: the status strip, a departure selected, the traffic list"
				class="block w-full"
				loading="lazy"
			/>
		</a>
		<div class="mt-6 flex flex-col gap-4 sm:flex-row sm:items-center">
			<div class="sm:w-96">{@render command(runMap, 'Copy the command that runs the airport map')}</div>
			<div class="flex flex-wrap gap-3">
				<a href="{base}/docs/examples" class="btn">Tour the map {@render arrow()}</a>
			</div>
		</div>
	</div>
</section>

<!-- Features -->
<section class="px-6 py-20" style="border-top: 1px solid var(--border);" aria-labelledby="features-heading">
	<div class="mx-auto max-w-6xl">
		<p class="eyebrow mb-3">Why SimConnect Go SDK</p>
		<h2 id="features-heading" class="mb-10 max-w-2xl text-2xl font-semibold tracking-tight sm:text-3xl" style="color: var(--text);">
			Build MSFS add&#8209;ons with Go
		</h2>
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			{#each features as f (f.title)}
				<div class="card p-5">
					<div class="mb-3 flex items-center gap-3">
						<span class="icon">
							<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">{@html f.icon}</svg>
						</span>
						<h3 class="text-[0.95rem] font-semibold" style="color: var(--text);">{f.title}</h3>
					</div>
					<p class="text-sm leading-relaxed" style="color: var(--text-2);">{f.description}</p>
				</div>
			{/each}
		</div>
		<p class="mt-10 max-w-2xl text-sm leading-relaxed" style="color: var(--text-2);">
			From real&#8209;time telemetry dashboards to&nbsp;AI&nbsp;traffic controllers&nbsp;&mdash; connect directly to&nbsp;the simulator with a&nbsp;typed, zero&#8209;dependency&nbsp;SDK.
		</p>
	</div>
</section>

<!-- Code -->
<section class="px-6 py-20" style="border-top: 1px solid var(--border);" aria-labelledby="code-preview-heading">
	<div class="mx-auto max-w-6xl">
		<p class="eyebrow mb-3">Two ways to connect</p>
		<h2 id="code-preview-heading" class="mb-3 text-2xl font-semibold tracking-tight sm:text-3xl" style="color: var(--text);">Client or manager</h2>
		<p class="mb-10 max-w-2xl text-sm leading-relaxed sm:text-base" style="color: var(--text-2);">
			Use the low-level client for full control, or the manager for production-ready lifecycle management.
		</p>
		<div class="grid gap-4 lg:grid-cols-2 [&>*]:min-w-0">
			{#each samples as s (s.label)}
				<div class="card overflow-hidden">
					<div class="px-4 py-2.5" style="border-bottom: 1px solid var(--border); background: var(--surface-2);">
						<span class="eyebrow">{s.label}</span>
					</div>
					<pre class="code"><code class="language-go">{@html s.html}</code></pre>
				</div>
			{/each}
		</div>
	</div>
</section>

<!-- Ecosystem -->
<section class="px-6 py-20" style="border-top: 1px solid var(--border);" aria-labelledby="ecosystem-heading">
	<div class="mx-auto max-w-6xl">
		<p class="eyebrow mb-3">Ecosystem</p>
		<h2 id="ecosystem-heading" class="mb-3 text-2xl font-semibold tracking-tight sm:text-3xl" style="color: var(--text);">Built on the same SDK</h2>
		<p class="mb-10 max-w-2xl text-sm leading-relaxed sm:text-base" style="color: var(--text-2);">
			More tools built on the same SDK — for AI-assisted development and terminal workflows.
		</p>
		<div class="grid gap-4 lg:grid-cols-2 [&>*]:min-w-0">
			<div class="card flex flex-col p-6">
				<div class="mb-3 flex flex-wrap items-center gap-3">
					<span class="icon">
						<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 17l6-6-6-6M12 19h8" /></svg>
					</span>
					<h3 class="text-base font-semibold" style="color: var(--text);">SimConnect MCP</h3>
					<span class="pill pill-brand">MCP server</span>
				</div>
				<p class="text-sm leading-relaxed" style="color: var(--text-2);">
					Let AI assistants query live simulator data and the full SimConnect SDK reference through the Model Context Protocol. Works with Claude Code, Claude Desktop, and any MCP-compatible client.
				</p>
				<div class="mt-5 flex flex-wrap gap-2">
					{#each ['1 800+ SimVars', 'Live data', 'Events', 'AI traffic', 'MSFS 2020 & 2024', '78 MCP tools'] as tag (tag)}
						<span class="pill">{tag}</span>
					{/each}
				</div>
				<div class="mt-auto flex flex-wrap gap-3 pt-6">
					<a href="https://simconnect-mcp.mrlm.net/" target="_blank" rel="noopener noreferrer" class="btn">View project {@render external()}</a>
					<a href="https://github.com/mrlm-net/simconnect-mcp" target="_blank" rel="noopener noreferrer" class="btn">GitHub</a>
				</div>
			</div>

			<div class="card flex flex-col p-6">
				<div class="mb-3 flex flex-wrap items-center gap-3">
					<span class="icon">
						<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2" /><path d="M7 9l3 3-3 3M13 15h4" /></svg>
					</span>
					<h3 class="text-base font-semibold" style="color: var(--text);">simvar-cli</h3>
					<span class="pill pill-brand">CLI tool</span>
				</div>
				<p class="text-sm leading-relaxed" style="color: var(--text-2);">
					Read, write, and stream MSFS simulation variables straight from the terminal. REPL, watch mode with CSV/JSON output, and a config file — zero extra installs beyond the Go toolchain.
				</p>
				<div class="term mt-5">
					<div class="term-head">$ simvar-cli</div>
					<div class="term-body">
						<div><span style="color: var(--text-3);">›</span> get <span class="str">"PLANE ALTITUDE"</span> feet float64</div>
						<div><span style="color: var(--text-3);">›</span> watch <span class="str">"AIRSPEED INDICATED"</span> knots float64</div>
						<div><span style="color: var(--text-3);">›</span> set <span class="str">"AUTOPILOT HEADING LOCK DIR"</span> degrees 270</div>
					</div>
				</div>
				<div class="mt-auto flex flex-wrap gap-3 pt-6">
					<a href="{base}/docs/simvar-cli" class="btn">Documentation {@render arrow()}</a>
					<a href="{siteConfig.repoUrl}/tree/main/cmd/simvar-cli" target="_blank" rel="noopener noreferrer" class="btn">GitHub</a>
				</div>
			</div>
		</div>
	</div>
</section>

<!-- Docs -->
<section class="px-6 py-20" style="border-top: 1px solid var(--border);" aria-labelledby="quick-links-heading">
	<div class="mx-auto max-w-6xl">
		<p class="eyebrow mb-3">Explore the docs</p>
		<h2 id="quick-links-heading" class="mb-10 text-2xl font-semibold tracking-tight sm:text-3xl" style="color: var(--text);">Get up and running in minutes</h2>
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			{#each quickLinks as d (d.href)}
				<a href={d.href} class="card group block p-5">
					<div class="mb-2 flex items-center justify-between text-sm font-semibold" style="color: var(--text);">
						{d.title}
						<span class="go" style="color: var(--text-3);">{@render arrow()}</span>
					</div>
					<p class="text-sm leading-relaxed" style="color: var(--text-2);">{d.description}</p>
				</a>
			{/each}
		</div>
	</div>
</section>

<!-- Commercial use + sponsor -->
<section class="px-6 pb-20">
	<div class="mx-auto grid max-w-6xl gap-4 md:grid-cols-2">
		<div class="card p-8">
			<p class="eyebrow mb-3">Business Source License 1.1</p>
			<h2 class="mb-3 text-xl font-semibold tracking-tight" style="color: var(--text);">Commercial use</h2>
			<p class="mb-6 text-sm leading-relaxed" style="color: var(--text-2);">
				Free for personal, hobby, community, education and non-profit use. A paid add-on or product, a paid service, or use inside a business needs a commercial licence — open an issue and we'll sort it out.
			</p>
			<a href="{siteConfig.repoUrl}/issues" target="_blank" rel="noopener noreferrer" class="btn btn-primary">Open an issue {@render external()}</a>
		</div>
		<div class="card p-8">
			<p class="eyebrow mb-3">Back open-source MSFS tooling</p>
			<h2 class="mb-3 flex items-center gap-2 text-xl font-semibold tracking-tight" style="color: var(--text);">
				<svg width="18" height="18" viewBox="0 0 24 24" fill="var(--danger)" aria-hidden="true"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" /></svg>
				Support the project
			</h2>
			<p class="mb-6 text-sm leading-relaxed" style="color: var(--text-2);">
				Sponsoring covers infrastructure costs, development time, and MSFS&nbsp;2020&nbsp;&amp;&nbsp;2024 licences required to test against real simulator versions.
			</p>
			<a href="https://revolut.me/mrlm?currency=EUR" target="_blank" rel="noopener noreferrer" class="btn">Sponsor via Revolut {@render external()}</a>
		</div>
	</div>
	<p class="mx-auto mt-10 max-w-6xl text-center text-sm" style="color: var(--text-3);">
		Microsoft Flight Simulator&nbsp;2020&nbsp;&amp;&nbsp;2024 &middot; Go&nbsp;1.27+ &middot; Windows &middot; Zero external dependencies
	</p>
</section>

<style>
	.hero {
		background:
			radial-gradient(ellipse 70% 60% at 85% 0%, var(--brand-soft) 0%, transparent 70%),
			var(--bg);
	}
	.icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		flex-shrink: 0;
		border-radius: 8px;
		background: var(--brand-soft);
		color: var(--brand);
	}
	.cmd {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.6rem 0.6rem 0.6rem 1rem;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--surface);
		box-shadow: var(--shadow-1);
	}
	.cmd code {
		flex: 1;
		min-width: 0;
		overflow-x: auto;
		white-space: nowrap;
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		color: var(--text);
		scrollbar-width: none;
	}
	.cmd code::-webkit-scrollbar {
		display: none;
	}
	.cmd button {
		display: inline-flex;
		padding: 0.4rem;
		border-radius: 6px;
		color: var(--text-3);
		cursor: pointer;
	}
	.cmd button:hover {
		color: var(--text);
		background: var(--surface-2);
	}
	.cmd button.ok {
		color: var(--ok);
	}
	.code {
		margin: 0;
		overflow-x: auto;
		padding: 1rem 1.15rem;
		background: var(--code-bg);
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		line-height: 1.7;
		color: var(--text);
	}
	.term {
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--code-bg);
		font-family: var(--font-mono);
		font-size: 0.75rem;
	}
	.term-head {
		padding: 0.4rem 0.75rem;
		border-bottom: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--text-3);
	}
	.term-body {
		padding: 0.6rem 0.75rem;
		color: var(--text-2);
		overflow-x: auto;
		white-space: nowrap;
	}
	.term-body .str {
		color: var(--tok-string);
	}
	.group:hover .go {
		color: var(--text) !important;
	}
</style>
