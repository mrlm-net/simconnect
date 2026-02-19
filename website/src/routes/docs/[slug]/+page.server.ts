import type { PageServerLoad, EntryGenerator } from './$types.js';
import { loadDocPage, loadAllSlugs } from '$lib/content/pipeline.js';
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

	return {
		doc
	};
};
