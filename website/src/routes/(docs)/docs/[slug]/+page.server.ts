import type { PageServerLoad, EntryGenerator } from './$types.js';
import { loadDocPage, loadDocIndex, loadAllSlugs } from '$lib/content/pipeline.server.js';
import { error } from '@sveltejs/kit';

export const prerender = true;

export const entries: EntryGenerator = () => {
	return loadAllSlugs().map((slug) => ({ slug }));
};

export const load: PageServerLoad = async ({ params }) => {
	const doc = await loadDocPage(params.slug);

	if (!doc) {
		error(404, 'Document not found');
	}

	const allDocs = loadDocIndex();
	allDocs.sort((a, b) => a.order - b.order);

	const idx = allDocs.findIndex((d) => d.slug === params.slug);
	const prev = idx > 0 ? { slug: allDocs[idx - 1].slug, title: allDocs[idx - 1].title } : null;
	const next =
		idx < allDocs.length - 1
			? { slug: allDocs[idx + 1].slug, title: allDocs[idx + 1].title }
			: null;

	return {
		doc,
		prev,
		next
	};
};
