import type { RequestHandler } from './$types.js';
import { loadDocIndex } from '$lib/content/pipeline.js';
import { siteConfig } from '$lib/config/site.js';

export const prerender = true;

export const GET: RequestHandler = () => {
	const baseUrl = `${siteConfig.url}${siteConfig.basePath}`;

	const staticPages = ['/', '/getting-started', '/docs', '/examples'];

	const docs = loadDocIndex();
	const docPages = docs.map((doc) => `/docs/${doc.slug}`);

	const allPages = [...staticPages, ...docPages];

	const urlEntries = allPages
		.map(
			(page) => `  <url>
    <loc>${baseUrl}${page}</loc>
  </url>`
		)
		.join('\n');

	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urlEntries}
</urlset>`;

	return new Response(xml, {
		headers: {
			'Content-Type': 'application/xml'
		}
	});
};
