import fs from 'node:fs';
import path from 'node:path';
import type { Example } from './types.js';

// The examples page follows docs/examples.md (#664): its sections in their
// order, the examples in each in the order of its tables, and what its
// tables say of each. The airport map comes first, as there. An example on
// disk that the doc does not list is shown last, with a build warning, so
// the two pages cannot drift apart unnoticed.

// The doc's section headings → the page's category keys (their colours).
const sectionKeys: Record<string, string> = {
	'The airport map': 'map',
	'Connection and lifecycle': 'basics',
	'Data and events': 'data',
	Facilities: 'facilities',
	Traffic: 'traffic',
	'Navigation and weather': 'nav',
	Spikes: 'spikes'
};

const categoryLabels: Record<string, string> = {
	map: 'Airport Map',
	basics: 'Connection and Lifecycle',
	data: 'Data and Events',
	facilities: 'Facilities',
	traffic: 'Traffic',
	nav: 'Navigation and Weather',
	spikes: 'Spikes',
	other: 'Other'
};

function repoDir(): string {
	return path.resolve(process.cwd(), '..');
}

function slugToTitle(slug: string): string {
	return slug
		.split('-')
		.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
		.join(' ');
}

// plain is a Markdown table cell or sentence as plain text.
function plain(md: string): string {
	return md
		.replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
		.replace(/<([^>]+)>/g, '$1')
		.replace(/\*\*|`/g, '')
		.replace(/\s+/g, ' ')
		.trim();
}

function firstSentence(text: string): string {
	const m = text.match(/^.*?\.(\s|$)/);
	return (m ? m[0] : text).trim();
}

interface Listed {
	slug: string;
	dir: string; // repository-relative: examples/<name> or cmd/<name>
	description: string;
	category: string;
}

// listed reads docs/examples.md: each example linked from a table row, and
// the airport map from the paragraph that introduces it.
function listed(): Listed[] {
	const file = path.join(repoDir(), 'docs', 'examples.md');
	if (!fs.existsSync(file)) return [];
	const md = fs.readFileSync(file, 'utf-8').replace(/\r\n/g, '\n');
	const out: Listed[] = [];
	let category = '';
	for (const line of md.split('\n')) {
		const h = line.match(/^#{2,3} (.+)$/);
		if (h) {
			category = sectionKeys[h[1].trim()] ?? category;
			continue;
		}
		if (category === 'map' && line.startsWith('[`cmd/airport-map`]') && !out.some((e) => e.slug === 'airport-map')) {
			out.push({ slug: 'airport-map', dir: 'cmd/airport-map', description: firstSentence(plain(line)).replace(/^cmd\/airport-map serves/, 'Serves'), category });
			continue;
		}
		const row = line.match(/^\|\s*\[([^\]]+)\]\(https:\/\/github\.com\/mrlm-net\/simconnect\/tree\/main\/([^)]+)\)\s*\|([^|]*)\|/);
		if (!row || !category) continue;
		const dir = row[2].replace(/\/$/, '');
		out.push({ slug: path.basename(dir), dir, description: plain(row[3]), category });
	}
	return out;
}

export function loadExamples(): Example[] {
	const root = repoDir();
	const examples: Example[] = [];
	const seen = new Set<string>();
	const read = (dir: string) => {
		const main = path.join(root, dir, 'main.go');
		return fs.existsSync(main) ? fs.readFileSync(main, 'utf-8').replace(/\r\n/g, '\n') : null;
	};

	for (const e of listed()) {
		const code = read(e.dir);
		if (code === null) {
			console.warn(`examples: docs/examples.md lists ${e.dir}, which has no main.go`);
			continue;
		}
		seen.add(e.dir);
		examples.push({ slug: e.slug, title: slugToTitle(e.slug), description: e.description, category: e.category, code });
	}

	// On disk but not in the doc: last, and said so.
	const dir = path.join(root, 'examples');
	if (fs.existsSync(dir)) {
		const rest = fs
			.readdirSync(dir, { withFileTypes: true })
			.filter((d) => d.isDirectory() && !seen.has(`examples/${d.name}`))
			.map((d) => d.name)
			.sort();
		for (const name of rest) {
			const code = read(`examples/${name}`);
			if (code === null) continue;
			console.warn(`examples: examples/${name} is not listed in docs/examples.md`);
			examples.push({ slug: name, title: slugToTitle(name), description: '', category: 'other', code });
		}
	}
	return examples;
}

export function getCategoryLabels(): Record<string, string> {
	return categoryLabels;
}
