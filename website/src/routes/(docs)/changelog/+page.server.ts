import type { PageServerLoad } from './$types.js';
import type { ChangelogRelease } from '$lib/types/index.js';
import { compile } from 'mdsvex';
import Prism from 'prismjs';
import 'prismjs/components/prism-go.js';
import 'prismjs/components/prism-bash.js';
import 'prismjs/components/prism-json.js';
import 'prismjs/components/prism-yaml.js';
import 'prismjs/components/prism-typescript.js';
import rehypeSlug from '$lib/plugins/rehype-slug.js';
import rehypeRewriteLinks from '$lib/plugins/rehype-rewrite-links.js';
import rehypeTableWrap from '$lib/plugins/rehype-table-wrap.js';

export const prerender = true;

interface GitHubRelease {
	tag_name: string;
	name: string;
	published_at: string;
	body: string;
	html_url: string;
	prerelease: boolean;
	draft: boolean;
}

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

/**
 * Resolve mdsvex {@html `...`} directives by extracting their template literal content.
 */
function resolveHtmlDirectives(code: string): string {
	return code.replace(/\{@html `([\s\S]*?)`\}/g, (_match, content) => {
		return content.replace(/\\`/g, '`');
	});
}

async function renderMarkdown(markdown: string): Promise<string> {
	if (!markdown.trim()) return '';

	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	const compiled = await compile(markdown, {
		highlight: { highlighter: highlightCode },
		rehypePlugins: [rehypeSlug, rehypeRewriteLinks, rehypeTableWrap]
	} as any);

	if (!compiled) return '';

	let code = compiled.code;
	code = code
		.replace(/<script[\s\S]*?<\/script>/gi, '')
		.replace(/<style[\s\S]*?<\/style>/gi, '');

	return resolveHtmlDirectives(code).trim();
}

export const load: PageServerLoad = async ({ fetch }) => {
	try {
		const response = await fetch(
			'https://api.github.com/repos/mrlm-net/simconnect/releases?per_page=100',
			{
				headers: {
					Accept: 'application/vnd.github.v3+json',
					'User-Agent': 'simconnect-docs'
				}
			}
		);

		if (!response.ok) {
			return { releases: [] };
		}

		const data: GitHubRelease[] = await response.json();

		const filtered = data.filter((r) => !r.draft);
		filtered.sort(
			(a, b) => new Date(b.published_at).getTime() - new Date(a.published_at).getTime()
		);

		const releases: ChangelogRelease[] = await Promise.all(
			filtered.map(async (r) => ({
				tag: r.tag_name,
				name: r.name || r.tag_name,
				date: r.published_at,
				body: r.body || '',
				renderedBody: await renderMarkdown(r.body || ''),
				url: r.html_url,
				prerelease: r.prerelease
			}))
		);

		return { releases };
	} catch {
		return { releases: [] };
	}
};
