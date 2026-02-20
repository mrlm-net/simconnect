import fs from 'node:fs';
import path from 'node:path';
import matter from 'gray-matter';
import { compile } from 'mdsvex';
import Prism from 'prismjs';
import 'prismjs/components/prism-go.js';
import 'prismjs/components/prism-bash.js';
import 'prismjs/components/prism-json.js';
import 'prismjs/components/prism-yaml.js';
import 'prismjs/components/prism-typescript.js';
import type { DocMeta } from '$lib/types/index.js';
import type { DocPage } from './types.js';
import { extractToc } from './toc.js';
import rehypeSlug from '$lib/plugins/rehype-slug.js';
import rehypeRewriteLinks from '$lib/plugins/rehype-rewrite-links.js';
import rehypeTableWrap from '$lib/plugins/rehype-table-wrap.js';

function escapeHtml(text: string): string {
	return text
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;');
}

function highlightCode(code: string, lang: string | undefined): string {
	if (lang && Prism.languages[lang]) {
		const html = Prism.highlight(code, Prism.languages[lang], lang);
		return `<pre class="language-${lang}"><code class="language-${lang}">${html}</code></pre>`;
	}
	return `<pre><code>${escapeHtml(code)}</code></pre>`;
}

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
		highlight: { highlighter: highlightCode },
		rehypePlugins: [rehypeSlug, rehypeRewriteLinks, rehypeTableWrap]
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
