import type { TocEntry } from '$lib/types/index.js';

export function slugify(text: string): string {
	return text
		.toLowerCase()
		.replace(/[^\w\s-]/g, '')
		.replace(/\s+/g, '-')
		.replace(/-+/g, '-')
		.trim();
}

export function extractToc(markdown: string): TocEntry[] {
	const entries: TocEntry[] = [];
	const headingRegex = /^(#{2})\s+(.+)$/gm;
	let match: RegExpExecArray | null;

	while ((match = headingRegex.exec(markdown)) !== null) {
		const depth = match[1].length;
		const text = match[2].trim();
		entries.push({
			depth,
			text,
			id: slugify(text)
		});
	}

	return entries;
}
