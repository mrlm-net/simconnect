<script lang="ts">
	import type { SiteConfig } from '$lib/types/index.js';

	let {
		siteConfig,
		title,
		description,
		path = '',
		type = 'website'
	}: {
		siteConfig: SiteConfig;
		title: string;
		description: string;
		path?: string;
		type?: string;
	} = $props();

	const pageTitle = $derived(
		title.includes(siteConfig.title) ? title : `${title} - ${siteConfig.title}`
	);
	const canonicalUrl = $derived(`${siteConfig.url}${siteConfig.basePath}${path}`);
	const ogImageUrl = $derived(`${siteConfig.url}${siteConfig.basePath}${siteConfig.ogImage}`);
</script>

<svelte:head>
	<title>{pageTitle}</title>
	<meta name="description" content={description} />
	<link rel="canonical" href={canonicalUrl} />

	<meta property="og:type" content={type} />
	<meta property="og:title" content={pageTitle} />
	<meta property="og:description" content={description} />
	<meta property="og:url" content={canonicalUrl} />
	<meta property="og:image" content={ogImageUrl} />
	<meta property="og:image:width" content={String(siteConfig.ogImageWidth)} />
	<meta property="og:image:height" content={String(siteConfig.ogImageHeight)} />
	<meta property="og:locale" content={siteConfig.locale} />
	<meta property="og:site_name" content={siteConfig.title} />

	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:title" content={pageTitle} />
	<meta name="twitter:description" content={description} />
	<meta name="twitter:image" content={ogImageUrl} />
</svelte:head>
