import type { RequestHandler } from './$types.js';
import { loadDocIndex } from '$lib/content/pipeline.js';
import { siteConfig } from '$lib/config/site.js';
import fs from 'node:fs';
import path from 'node:path';

export const prerender = true;

function toW3CDate(date: Date): string {
	return date.toISOString().split('T')[0];
}

function docLastmod(slug: string): string {
	const filePath = path.resolve(process.cwd(), '..', 'docs', `${slug}.md`);
	try {
		const stat = fs.statSync(filePath);
		return toW3CDate(stat.mtime);
	} catch {
		return toW3CDate(new Date());
	}
}

interface SitemapEntry {
	path: string;
	priority: string;
	lastmod: string;
}

export const GET: RequestHandler = () => {
	const baseUrl = `${siteConfig.url}${siteConfig.basePath}`;
	const buildDate = toW3CDate(new Date());

	const staticPages: SitemapEntry[] = [
		{ path: '/', priority: '1.0', lastmod: buildDate },
		{ path: '/getting-started', priority: '0.9', lastmod: buildDate },
		{ path: '/docs', priority: '0.8', lastmod: buildDate },
		{ path: '/examples', priority: '0.8', lastmod: buildDate },
		{ path: '/changelog', priority: '0.7', lastmod: buildDate }
	];

	const docs = loadDocIndex();
	const docPages: SitemapEntry[] = docs.map((doc) => ({
		path: `/docs/${doc.slug}`,
		priority: '0.7',
		lastmod: docLastmod(doc.slug)
	}));

	const allPages = [...staticPages, ...docPages];

	const urlEntries = allPages
		.map(
			(entry) => `  <url>
    <loc>${baseUrl}${entry.path}</loc>
    <lastmod>${entry.lastmod}</lastmod>
    <priority>${entry.priority}</priority>
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
