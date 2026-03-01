import type { DocMeta, NavSection } from '$lib/types/index.js';

const sectionMeta: Record<string, { title: string; defaultOpen: boolean }> = {
	client: { title: 'Client / Engine', defaultOpen: true },
	manager: { title: 'Manager', defaultOpen: true },
	datasets: { title: 'Datasets', defaultOpen: true },
	traffic: { title: 'Traffic', defaultOpen: true },
	events: { title: 'Events', defaultOpen: true },
	internals: { title: 'Internals', defaultOpen: false }
};

const sectionOrder = ['client', 'manager', 'datasets', 'traffic', 'events', 'internals'];

export function buildNavigation(docs: DocMeta[], basePath: string): NavSection[] {
	const grouped = new Map<string, DocMeta[]>();

	for (const doc of docs) {
		const list = grouped.get(doc.section) ?? [];
		list.push(doc);
		grouped.set(doc.section, list);
	}

	return sectionOrder
		.filter((id) => grouped.has(id))
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
