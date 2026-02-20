import type { PageServerLoad } from './$types.js';
import { loadDocIndex } from '$lib/content/pipeline.server.js';

export const prerender = true;

export const load: PageServerLoad = () => {
	const docs = loadDocIndex();
	docs.sort((a, b) => a.order - b.order);
	return { docs };
};
