import type { TocEntry } from '$lib/types/index.js';

export interface DocPage {
	slug: string;
	title: string;
	description: string;
	order: number;
	section: string;
	renderedContent: string;
	headings: TocEntry[];
}

export interface Example {
	slug: string;
	title: string;
	description: string;
	category: string;
	code: string;
}