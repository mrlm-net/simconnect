<script lang="ts">
	import { base } from '$app/paths';
	import Prism from 'prismjs';
	import 'prismjs/components/prism-go';

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
			description:
				'Standard library only. No external packages to manage, audit, or update.'
		},
		{
			title: 'Fully Typed API',
			description:
				'Strong typing with dedicated structs, enums, and typed constants throughout.'
		},
		{
			title: 'Auto-Reconnect',
			description:
				'Built-in connection lifecycle management with automatic reconnection support.'
		},
		{
			title: 'Ready-Made Datasets',
			description:
				'Pre-built dataset definitions for aircraft, environment, facilities, and traffic.'
		}
	];

	const quickLinks = [
		{
			title: 'Getting Started',
			description: 'Step-by-step guide to your first connection',
			href: `${base}/getting-started`
		},
		{
			title: 'Client API',
			description: 'Direct SimConnect communication reference',
			href: `${base}/docs/usage-client`
		},
		{
			title: 'Manager API',
			description: 'Auto-reconnect and lifecycle management',
			href: `${base}/docs/usage-manager`
		}
	];
</script>

<svelte:head>
	<title>SimConnect Go SDK -- GoLang wrapper for MSFS 2020/2024</title>
	<meta
		name="description"
		content="Build Microsoft Flight Simulator add-ons with Go. Lightweight, typed, zero-dependency SimConnect wrapper."
	/>
</svelte:head>

<!-- Hero Section -->
<section class="py-16 md:py-24">
	<div class="mx-auto max-w-4xl px-6 text-center">
		<h1 class="mb-4 text-4xl font-bold tracking-tight md:text-5xl" style="color: var(--color-text-primary);">
			SimConnect Go SDK
		</h1>
		<p class="mb-2 text-xl md:text-2xl" style="color: var(--color-link);">
			Build Microsoft Flight Simulator add-ons with Go
		</p>
		<p class="mx-auto mb-8 max-w-2xl text-lg" style="color: var(--color-text-secondary);">
			Lightweight, typed, zero-dependency wrapper over SimConnect.dll for MSFS 2020 and 2024.
		</p>

		<div class="mb-8 flex flex-col items-center justify-center gap-4 sm:flex-row">
			<a
				href="{base}/getting-started"
				class="inline-flex items-center rounded-lg px-6 py-3 text-base font-medium transition-colors"
				style="background-color: var(--color-link); color: var(--color-bg-primary);"
			>
				Get Started
			</a>
			<a
				href="{base}/docs"
				class="inline-flex items-center rounded-lg border px-6 py-3 text-base font-medium transition-colors"
				style="border-color: var(--color-border); color: var(--color-text-primary);"
			>
				Documentation
			</a>
		</div>

		<div
			class="mx-auto inline-block rounded-lg border px-5 py-3 font-mono text-sm"
			style="background-color: var(--color-bg-code); border-color: var(--color-border); color: var(--color-text-secondary);"
		>
			<span style="color: var(--color-text-muted);">$</span>
			<span style="color: var(--color-text-primary);"> go get github.com/mrlm-net/simconnect</span>
		</div>
	</div>
</section>

<!-- Feature Cards -->
<section class="py-12" style="border-top: 1px solid var(--color-border);" aria-labelledby="features-heading">
	<div class="mx-auto max-w-4xl px-6">
		<h2 id="features-heading" class="sr-only">Features</h2>
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			{#each features as feature}
				<div
					class="rounded-lg border p-6"
					style="background-color: var(--color-bg-secondary); border-color: var(--color-border);"
				>
					<h3 class="mb-2 text-base font-semibold" style="color: var(--color-text-primary);">
						{feature.title}
					</h3>
					<p class="text-sm" style="color: var(--color-text-secondary);">
						{feature.description}
					</p>
				</div>
			{/each}
		</div>
	</div>
</section>

<!-- Code Preview -->
<section class="py-12" style="border-top: 1px solid var(--color-border);" aria-labelledby="code-preview-heading">
	<div class="mx-auto max-w-5xl px-6">
		<h2 id="code-preview-heading" class="sr-only">Code Examples</h2>
		<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
			<div
				class="overflow-hidden rounded-lg border"
				style="background-color: var(--color-bg-code); border-color: var(--color-border);"
			>
				<div
					class="px-4 py-2 font-mono text-xs uppercase"
					style="background-color: var(--color-bg-tertiary); color: var(--color-text-muted);"
				>
					Low-Level Client
				</div>
				<pre class="overflow-x-auto p-4"><code class="language-go text-[0.8125rem] leading-relaxed">{@html clientHighlighted}</code></pre>
			</div>

			<div
				class="overflow-hidden rounded-lg border"
				style="background-color: var(--color-bg-code); border-color: var(--color-border);"
			>
				<div
					class="px-4 py-2 font-mono text-xs uppercase"
					style="background-color: var(--color-bg-tertiary); color: var(--color-text-muted);"
				>
					Manager with Auto-Reconnect
				</div>
				<pre class="overflow-x-auto p-4"><code class="language-go text-[0.8125rem] leading-relaxed">{@html managerHighlighted}</code></pre>
			</div>
		</div>
	</div>
</section>

<!-- Quick Links -->
<section class="py-12" style="border-top: 1px solid var(--color-border);" aria-labelledby="quick-links-heading">
	<div class="mx-auto max-w-4xl px-6">
		<h2 id="quick-links-heading" class="sr-only">Quick Links</h2>
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			{#each quickLinks as link}
				<a
					href={link.href}
					class="group rounded-lg border p-5 transition-colors"
					style="background-color: var(--color-bg-secondary); border-color: var(--color-border);"
				>
					<h3 class="mb-1 flex items-center gap-2 text-base font-semibold" style="color: var(--color-text-primary);">
						{link.title}
						<span
							class="transition-transform group-hover:translate-x-0.5"
							style="color: var(--color-text-muted);"
						>
							&rarr;
						</span>
					</h3>
					<p class="text-sm" style="color: var(--color-text-secondary);">
						{link.description}
					</p>
				</a>
			{/each}
		</div>
	</div>
</section>

<!-- Compatibility Footer -->
<section class="py-8" style="border-top: 1px solid var(--color-border);">
	<div class="mx-auto max-w-4xl px-6 text-center">
		<p class="text-sm" style="color: var(--color-text-muted);">
			Works with Microsoft Flight Simulator 2020 and 2024 &middot; Requires Go 1.25+ and Windows
		</p>
	</div>
</section>

<style>
	a.group:hover {
		border-color: var(--color-link) !important;
	}
</style>
