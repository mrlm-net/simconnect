import type { RequestHandler } from './$types.js';
import { loadDocIndex } from '$lib/content/pipeline.js';
import { siteConfig } from '$lib/config/site.js';

export const prerender = true;

export const GET: RequestHandler = () => {
	const baseUrl = `${siteConfig.url}${siteConfig.basePath}`;
	const docs = loadDocIndex();
	docs.sort((a, b) => a.order - b.order);

	const docList = docs.map((doc) => `- [${doc.title}](${baseUrl}/docs/${doc.slug}): ${doc.description}`).join('\n');

	const content = `# ${siteConfig.title}

> ${siteConfig.description}

## Key URLs

- Website: ${baseUrl}/
- Getting Started: ${baseUrl}/getting-started
- Documentation: ${baseUrl}/docs
- Examples: ${baseUrl}/examples
- Repository: ${siteConfig.repoUrl}
- Go Reference: https://pkg.go.dev/github.com/mrlm-net/simconnect

## Documentation

${docList}
`;

	return new Response(content, {
		headers: {
			'Content-Type': 'text/plain; charset=utf-8'
		}
	});
};
