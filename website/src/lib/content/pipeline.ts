import fs from 'node:fs';
import path from 'node:path';
import matter from 'gray-matter';
import { compile } from 'mdsvex';
import type { DocMeta, TocEntry } from '$lib/types/index.js';
import { extractToc } from './toc.js';
import rehypeHighlight from '$lib/plugins/rehype-highlight.js';
import rehypeSlug from '$lib/plugins/rehype-slug.js';
import rehypeRewriteLinks from '$lib/plugins/rehype-rewrite-links.js';

function docsDir(): string {
	return path.resolve(process.cwd(), '..', 'docs');
}

function slugFromFilename(filename: string): string {
	return filename.replace(/\.md$/, '');
}

export function loadDocIndex(): DocMeta[] {
	const dir = docsDir();
	const files = fs.readdirSync(dir).filter((f: string) => f.endsWith('.md'));

	return files.map((file: string) => {
		const raw = fs.readFileSync(path.join(dir, file), 'utf-8');
		const { data } = matter(raw);
		return {
			slug: slugFromFilename(file),
			title: (data.title as string) ?? slugFromFilename(file),
			description: (data.description as string) ?? '',
			order: (data.order as number) ?? 99,
			section: (data.section as string) ?? 'general'
		};
	});
}

export interface DocPage {
	slug: string;
	title: string;
	description: string;
	order: number;
	section: string;
	renderedContent: string;
	headings: TocEntry[];
}

/**
 * Resolve mdsvex {@html `...`} directives by extracting their template literal content.
 * mdsvex wraps code blocks in {@html `<pre>...</pre>`} which are Svelte template directives
 * that need to be unwrapped for static HTML rendering.
 */
function resolveHtmlDirectives(code: string): string {
	// Match {@html `...`} blocks — the backtick-delimited content is the raw HTML
	return code.replace(/\{@html `([\s\S]*?)`\}/g, (_match, content) => {
		// The content inside template literals may have escaped backticks
		return content.replace(/\\`/g, '`');
	});
}

export async function loadDocPage(slug: string): Promise<DocPage | null> {
	const filePath = path.join(docsDir(), `${slug}.md`);
	if (!fs.existsSync(filePath)) {
		return null;
	}

	const raw = fs.readFileSync(filePath, 'utf-8');
	const { data, content } = matter(raw);

	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	const compiled = await compile(content, {
		rehypePlugins: [
			rehypeSlug,
			rehypeRewriteLinks,
			[
				rehypeHighlight,
				{ languages: ['go', 'bash', 'json', 'yaml', 'javascript', 'typescript'] }
			]
		]
	} as any);

	let renderedContent = '';
	if (compiled) {
		let code = compiled.code;
		// Remove script and style blocks from the Svelte component
		code = code
			.replace(/<script[\s\S]*?<\/script>/gi, '')
			.replace(/<style[\s\S]*?<\/style>/gi, '');
		// Resolve Svelte {@html `...`} directives to plain HTML
		renderedContent = resolveHtmlDirectives(code).trim();
	}

	return {
		slug,
		title: (data.title as string) ?? slug,
		description: (data.description as string) ?? '',
		order: (data.order as number) ?? 99,
		section: (data.section as string) ?? 'general',
		renderedContent,
		headings: extractToc(content)
	};
}

export function loadAllSlugs(): string[] {
	const dir = docsDir();
	return fs
		.readdirSync(dir)
		.filter((f: string) => f.endsWith('.md'))
		.map(slugFromFilename);
}
