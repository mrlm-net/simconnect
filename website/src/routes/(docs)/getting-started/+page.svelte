<script lang="ts">
	import { base } from '$app/paths';
	import TableOfContents from '$lib/components/layout/TableOfContents.svelte';
	import Prism from 'prismjs';
	import 'prismjs/components/prism-clike';
	import 'prismjs/components/prism-go';
	import 'prismjs/components/prism-bash';

	const headings = [
		{ depth: 2, text: 'Prerequisites', id: 'prerequisites' },
		{ depth: 2, text: 'Installation', id: 'installation' },
		{ depth: 2, text: 'Your First Connection', id: 'your-first-connection' },
		{ depth: 2, text: 'Using the Manager', id: 'using-the-manager' },
		{ depth: 2, text: 'Next Steps', id: 'next-steps' }
	];

	function escapeHtml(text: string): string {
		return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
	}

	function hl(code: string, lang: string): string {
		const grammar = Prism.languages[lang];
		if (grammar) return Prism.highlight(code, grammar, lang);
		return escapeHtml(code);
	}

	const installCode = `mkdir my-simconnect-app && cd my-simconnect-app
go mod init my-simconnect-app
go get github.com/mrlm-net/simconnect`;

	const firstConnectionCode = `//go:build windows

package main

import (
    "fmt"
    "log"
    "time"

    "github.com/mrlm-net/simconnect"
)

func main() {
    client := simconnect.NewClient("My First App")

    if err := client.Connect(); err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer client.Disconnect()

    fmt.Println("Connected to SimConnect!")
    time.Sleep(5 * time.Second)
    fmt.Println("Disconnecting...")
}`;

	const managerCode = `//go:build windows

package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"

    "github.com/mrlm-net/simconnect/pkg/manager"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt)

    mgr := manager.New("My Managed App",
        manager.WithContext(ctx),
        manager.WithAutoReconnect(true),
    )

    mgr.OnConnectionStateChange(
        func(old, current manager.ConnectionState) {
            fmt.Printf("State: %s -> %s\\n", old, current)
        },
    )

    go func() {
        <-sigChan
        fmt.Println("Shutting down...")
        mgr.Stop()
        cancel()
    }()

    if err := mgr.Start(); err != nil {
        fmt.Println("Manager stopped:", err)
    }
}`;

	const installHighlighted = hl(installCode, 'bash');
	const firstConnectionHighlighted = hl(firstConnectionCode, 'go');
	const managerHighlighted = hl(managerCode, 'go');

	const nextSteps = [
		{
			title: 'Client Configuration',
			description: 'All client options and settings',
			href: `${base}/docs/config-client`
		},
		{
			title: 'Manager Usage',
			description: 'Full manager API reference',
			href: `${base}/docs/usage-manager`
		},
		{
			title: 'Examples',
			description: 'Browse 25+ example applications',
			href: 'https://github.com/mrlm-net/simconnect/tree/main/examples',
			external: true
		}
	];
</script>

<svelte:head>
	<title>Getting Started -- SimConnect Go SDK</title>
	<meta
		name="description"
		content="Step-by-step guide to connect your Go application to Microsoft Flight Simulator."
	/>
</svelte:head>

<div class="flex">
	<article class="prose mx-auto min-w-0 max-w-4xl flex-1 p-6 lg:p-10">
		<h1>Getting Started</h1>
		<p>
			This guide walks you through installing the SimConnect Go SDK and connecting to Microsoft
			Flight Simulator for the first time.
		</p>

		<h2 id="prerequisites">Prerequisites</h2>
		<p>Before you begin, make sure you have the following:</p>
		<ul>
			<li>
				<strong>Microsoft Flight Simulator</strong> 2020 or 2024
			</li>
			<li>
				<strong>SimConnect SDK</strong> (included with the MSFS SDK)
			</li>
			<li>
				<strong>Go 1.25+</strong> &mdash; download from
				<a href="https://go.dev/dl/" target="_blank" rel="noopener noreferrer">go.dev/dl/</a>
			</li>
			<li>
				<strong>Windows OS</strong> (SimConnect is a Windows-only API)
			</li>
		</ul>

		<h2 id="installation">Installation</h2>
		<p>Create a new Go project and install the SDK:</p>
		<pre><code class="language-bash">{@html installHighlighted}</code></pre>

		<blockquote>
			<p>
				All SimConnect code requires the <code>windows</code> build tag. Files must include
				<code>{"//go:build windows"}</code> at the top.
			</p>
		</blockquote>

		<h2 id="your-first-connection">Your First Connection</h2>
		<p>
			Create a <code>main.go</code> file with the following code:
		</p>
		<pre><code class="language-go">{@html firstConnectionHighlighted}</code></pre>

		<p>Run your application while MSFS is running:</p>
		<pre><code class="language-bash">{@html hl('go run .', 'bash')}</code></pre>

		<blockquote>
			<p>
				<strong>Troubleshooting:</strong> If you get a DLL not found error, use
				<code>simconnect.ClientWithDLLPath("path/to/SimConnect.dll")</code> or set the
				<code>SIMCONNECT_DLL</code> environment variable.
			</p>
		</blockquote>

		<h2 id="using-the-manager">Using the Manager</h2>
		<p>
			For production applications, the Manager provides automatic reconnection, state tracking,
			and structured lifecycle management. This is the recommended approach for robust add-ons.
		</p>
		<pre><code class="language-go">{@html managerHighlighted}</code></pre>

		<p>Key differences from the low-level client:</p>
		<ul>
			<li>
				<strong>Auto-reconnect:</strong> The manager automatically retries the connection when
				the simulator disconnects or is not running.
			</li>
			<li>
				<strong>State callbacks:</strong> Subscribe to connection state changes instead of
				polling.
			</li>
			<li>
				<strong>Graceful shutdown:</strong> Signal handling with context cancellation ensures
				clean disconnection.
			</li>
		</ul>

		<h2 id="next-steps">Next Steps</h2>
		<div class="not-prose grid grid-cols-1 gap-4 sm:grid-cols-3">
			{#each nextSteps as link}
				<a
					href={link.href}
					target={link.external ? '_blank' : undefined}
					rel={link.external ? 'noopener noreferrer' : undefined}
					class="group rounded-lg border p-5 transition-colors"
					style="background-color: var(--color-bg-secondary); border-color: var(--color-border);"
				>
					<h3
						class="mb-1 flex items-center gap-2 text-base font-semibold"
						style="color: var(--color-text-primary);"
					>
						{link.title}
						<span
							class="transition-transform group-hover:translate-x-0.5"
							style="color: var(--color-text-muted);"
						>
							{link.external ? '\u2197' : '\u2192'}
						</span>
					</h3>
					<p class="text-sm" style="color: var(--color-text-secondary);">
						{link.description}
					</p>
				</a>
			{/each}
		</div>
	</article>

	<aside class="hidden shrink-0 pr-4 xl:block">
		<TableOfContents {headings} />
	</aside>
</div>

<style>
	a.group:hover {
		border-color: var(--color-link) !important;
	}
</style>
