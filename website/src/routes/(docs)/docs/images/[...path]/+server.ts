import fs from 'node:fs';
import path from 'node:path';
import { error } from '@sveltejs/kit';
import type { EntryGenerator, RequestHandler } from './$types.js';

// Images of docs/*.md live in docs/images, next to the markdown, so they show
// on GitHub too. They are copied into the site at build time from here.

export const prerender = true;

const types: Record<string, string> = {
	'.png': 'image/png',
	'.jpg': 'image/jpeg',
	'.jpeg': 'image/jpeg',
	'.gif': 'image/gif',
	'.svg': 'image/svg+xml',
	'.webp': 'image/webp'
};

function imagesDir(): string {
	return path.resolve(process.cwd(), '..', 'docs', 'images');
}

function list(dir: string, prefix = ''): string[] {
	if (!fs.existsSync(dir)) return [];
	return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
		const rel = prefix ? `${prefix}/${e.name}` : e.name;
		if (e.isDirectory()) return list(path.join(dir, e.name), rel);
		return types[path.extname(e.name).toLowerCase()] ? [rel] : [];
	});
}

export const entries: EntryGenerator = () => list(imagesDir()).map((p) => ({ path: p }));

export const GET: RequestHandler = ({ params }) => {
	const root = imagesDir();
	const file = path.resolve(root, params.path);
	const type = types[path.extname(file).toLowerCase()];
	if (!file.startsWith(root + path.sep) || !type || !fs.existsSync(file)) {
		error(404, 'Not found');
	}
	return new Response(fs.readFileSync(file), { headers: { 'Content-Type': type } });
};
