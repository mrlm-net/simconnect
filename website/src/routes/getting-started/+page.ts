import { redirect } from '@sveltejs/kit';
import { base } from '$app/paths';

// Getting Started is one page, in the docs: the old address leads there.
export const prerender = true;

export function load() {
	redirect(301, `${base}/docs/getting-started`);
}
