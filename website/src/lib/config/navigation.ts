import type { DocMeta, NavSection } from '$lib/types/index.js';

export const sectionMeta: Record<string, { title: string; defaultOpen: boolean }> = {
	examples: { title: 'Examples', defaultOpen: true },
	client: { title: 'Client / Engine', defaultOpen: true },
	manager: { title: 'Manager', defaultOpen: true },
	nav: { title: 'Airport & Navigation', defaultOpen: true },
	traffic: { title: 'Traffic', defaultOpen: true },
	packages: { title: 'Packages', defaultOpen: true },
	tools: { title: 'Tools', defaultOpen: true }
};

export const sectionOrder = ['examples', 'client', 'manager', 'nav', 'traffic', 'packages', 'tools'];

// orderedSections: the known sections in sectionOrder, then any other a doc names
// (so a page in a new section is never left out of the navigation).
export function orderedSections(ids: Iterable<string>): string[] {
	const present = new Set(ids);
	const known = sectionOrder.filter((id) => present.has(id));
	const others = [...present].filter((id) => !sectionOrder.includes(id)).sort();
	return [...known, ...others];
}

export function buildNavigation(docs: DocMeta[], basePath: string): NavSection[] {
	const grouped = new Map<string, DocMeta[]>();

	for (const doc of docs) {
		const list = grouped.get(doc.section) ?? [];
		list.push(doc);
		grouped.set(doc.section, list);
	}

	return orderedSections(grouped.keys())
		.map((id) => {
			const items = grouped.get(id)!;
			items.sort((a, b) => a.order - b.order);
			const meta = sectionMeta[id] ?? { title: id, defaultOpen: false };
			return {
				title: meta.title,
				id,
				defaultOpen: meta.defaultOpen,
				items: items.map((doc) => ({
					title: doc.title,
					href: `${basePath}/docs/${doc.slug}`,
					order: doc.order
				}))
			};
		});
}
