import type { LayoutServerLoad } from './$types.js';
import { loadDocIndex } from '$lib/content/pipeline.server.js';
import { buildNavigation } from '$lib/config/navigation.js';
import { siteConfig } from '$lib/config/site.js';
import { base } from '$app/paths';

export const prerender = true;

export const load: LayoutServerLoad = () => {
	const docs = loadDocIndex();
	const navigation = buildNavigation(docs, base);
	const topLinks = [
		{ title: 'Getting Started', href: `${base}/getting-started`, order: 0 },
		{ title: 'Examples', href: `${base}/examples`, order: 1 },
		{ title: 'Changelog', href: `${base}/changelog`, order: 2 }
	];

	return {
		navigation,
		topLinks,
		siteConfig
	};
};
