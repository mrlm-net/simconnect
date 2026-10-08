export interface NavItem {
	title: string;
	href: string;
	order: number;
}

export interface NavSection {
	title: string;
	id: string;
	items: NavItem[];
	defaultOpen?: boolean;
}

export interface TocEntry {
	depth: number;
	text: string;
	id: string;
}

export interface SiteConfig {
	title: string;
	description: string;
	repoUrl: string;
	basePath: string;
	url: string;
	ogImage: string;
	ogImageWidth: number;
	ogImageHeight: number;
	locale: string;
	license: string;
	/** Short label for the license pill, e.g. "BSL 1.1 · non-commercial". */
	licenseLabel: string;
	/** First year of the copyright line. */
	since: number;
	/** One or two characters in the brand mark. */
	glyph: string;
	/** Header links; match is the path prefix that marks a link active (default href). */
	nav: { title: string; href: string; match?: string }[];
}

export interface DocMeta {
	slug: string;
	title: string;
	description: string;
	order: number;
	section: string;
}

export interface ChangelogRelease {
	tag: string;
	name: string;
	date: string;
	body: string;
	renderedBody: string;
	url: string;
	prerelease: boolean;
}
