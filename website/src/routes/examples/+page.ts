import { redirect } from '@sveltejs/kit';
import { base } from '$app/paths';

// The examples are one page, in the docs (#664): the old address leads there.
export const prerender = true;

export function load() {
	redirect(301, `${base}/docs/examples`);
}
