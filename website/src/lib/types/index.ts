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
