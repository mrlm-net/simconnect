import type { SiteConfig } from '$lib/types/index.js';

export const siteConfig: SiteConfig = {
	title: 'SimConnect Go SDK',
	description: 'GoLang wrapper over SimConnect.dll for MSFS 2020/2024',
	repoUrl: 'https://github.com/mrlm-net/simconnect',
	basePath: '',
	url: 'https://simconnect.mrlm.net',
	ogImage: '/hero.png',
	ogImageWidth: 1057,
	ogImageHeight: 639,
	locale: 'en_US',
	license: 'BUSL-1.1',
	licenseLabel: 'BSL 1.1 · non-commercial',
	since: 2025,
	glyph: 'sc',
	nav: [
		{ title: 'Docs', href: '/docs' },
		{ title: 'Getting started', href: '/getting-started' },
		{ title: 'Examples', href: '/docs/examples' },
		{ title: 'Changelog', href: '/changelog' }
	]
};
