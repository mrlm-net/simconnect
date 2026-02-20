import type { PageServerLoad } from './$types.js';
import { loadExamples, getCategoryLabels } from '$lib/content/examples.server.js';

export const prerender = true;

export const load: PageServerLoad = () => {
	const examples = loadExamples();
	const categoryLabels = getCategoryLabels();
	return { examples, categoryLabels };
};
