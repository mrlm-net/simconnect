import fs from 'node:fs';
import path from 'node:path';
import type { Example } from './types.js';

const categoryMap: Record<string, string> = {
	'basic-connection': 'basics',
	'lifecycle-connection': 'basics',
	'await-connection': 'basics',
	'read-messages': 'data',
	'read-objects': 'data',
	'set-variables': 'data',
	'using-datasets': 'data',
	'emit-events': 'events',
	'subscribe-events': 'events',
	'read-facility': 'facilities',
	'read-facilities': 'facilities',
	'subscribe-facilities': 'facilities',
	'all-facilities': 'facilities',
	'airport-details': 'facilities',
	'locate-airport': 'facilities',
	'read-waypoints': 'facilities',
	'ai-traffic': 'traffic',
	'manage-traffic': 'traffic',
	'monitor-traffic': 'traffic',
	'simconnect-manager': 'manager',
	'simconnect-subscribe': 'manager',
	'simconnect-state': 'manager',
	'simconnect-events': 'manager',
	'simconnect-facilities': 'manager',
	'simconnect-traffic': 'manager',
	'simconnect-benchmark': 'manager'
};

const categoryLabels: Record<string, string> = {
	basics: 'Getting Started',
	data: 'Data & Objects',
	events: 'Events',
	facilities: 'Facilities',
	traffic: 'AI Traffic',
	manager: 'Manager'
};

function slugToTitle(slug: string): string {
	return slug
		.split('-')
		.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
		.join(' ');
}

function extractDescription(code: string): string {
	const lines = code.split('\n');
	for (const line of lines) {
		const trimmed = line.trim();
		if (trimmed.startsWith('//') && !trimmed.startsWith('//go:build') && !trimmed.startsWith('// +build')) {
			return trimmed.replace(/^\/\/\s*/, '');
		}
	}
	return '';
}

function examplesDir(): string {
	return path.resolve(process.cwd(), '..', 'examples');
}

export function loadExamples(): Example[] {
	const dir = examplesDir();
	if (!fs.existsSync(dir)) return [];

	const entries = fs.readdirSync(dir, { withFileTypes: true });
	const examples: Example[] = [];

	for (const entry of entries) {
		if (!entry.isDirectory()) continue;

		const mainPath = path.join(dir, entry.name, 'main.go');
		if (!fs.existsSync(mainPath)) continue;

		const code = fs.readFileSync(mainPath, 'utf-8');
		const slug = entry.name;

		examples.push({
			slug,
			title: slugToTitle(slug),
			description: extractDescription(code),
			category: categoryMap[slug] ?? 'other',
			code
		});
	}

	examples.sort((a, b) => {
		const catOrder = Object.keys(categoryMap);
		const ai = catOrder.indexOf(a.slug);
		const bi = catOrder.indexOf(b.slug);
		return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
	});

	return examples;
}

export function getCategoryLabels(): Record<string, string> {
	return categoryLabels;
}
