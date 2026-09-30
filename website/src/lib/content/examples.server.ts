import fs from 'node:fs';
import path from 'node:path';
import type { Example } from './types.js';

// The order here is the order on the examples page: the airport map first.
const categoryMap: Record<string, string> = {
	'airport-map': 'map',
	'basic-connection': 'basics',
	'lifecycle-connection': 'basics',
	'await-connection': 'basics',
	'read-messages': 'data',
	'read-objects': 'data',
	'set-variables': 'data',
	'using-datasets': 'data',
	'emit-events': 'events',
	'subscribe-events': 'events',
	'flow-events': 'events',
	'read-facility': 'facilities',
	'read-facilities': 'facilities',
	'subscribe-facilities': 'facilities',
	'all-facilities': 'facilities',
	'airport-details': 'facilities',
	'locate-airport': 'facilities',
	'read-waypoints': 'facilities',
	'ai-taxi': 'traffic',
	'ai-arrival': 'traffic',
	'ai-traffic': 'traffic',
	'manage-traffic': 'traffic',
	'monitor-traffic': 'traffic',
	atis: 'nav',
	'flight-plan': 'nav',
	'spike-airways': 'nav',
	'simconnect-manager': 'manager',
	'simconnect-subscribe': 'manager',
	'simconnect-state': 'manager',
	'simconnect-events': 'manager',
	'simconnect-facilities': 'manager',
	'simconnect-traffic': 'manager',
	'simconnect-benchmark': 'manager',
	'simvar-cli': 'tools'
};

const categoryLabels: Record<string, string> = {
	map: 'Airport Map',
	basics: 'Getting Started',
	data: 'Data & Objects',
	events: 'Events',
	facilities: 'Facilities',
	traffic: 'AI Traffic',
	nav: 'Navigation',
	manager: 'Manager',
	tools: 'Tools',
	spikes: 'Spikes'
};

function slugToTitle(slug: string): string {
	return slug
		.split('-')
		.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
		.join(' ');
}

function firstSentence(text: string): string {
	const m = text.match(/^.*?\.(\s|$)/);
	return (m ? m[0] : text).trim();
}

// The first sentence of the doc comment above `package main`.
function docComment(code: string): string {
	let lines: string[] = [];
	for (const line of code.split('\n')) {
		const t = line.trim();
		if (t.startsWith('package ')) break;
		if (t.startsWith('//go:build') || t.startsWith('// +build')) continue;
		if (t.startsWith('//')) lines.push(line.replace(/^\s*\/\/ ?/, ''));
		else lines = []; // a blank line ends a comment block
	}
	const text = lines
		.filter((l) => !l.startsWith('\t')) // indented: commands, not prose
		.join(' ')
		.replace(/\s+/g, ' ')
		.trim();
	return firstSentence(text);
}

// The first sentence of the README's overview (or of its first paragraph).
function readmeSummary(dir: string): string {
	const file = path.join(dir, 'README.md');
	if (!fs.existsSync(file)) return '';
	const md = fs.readFileSync(file, 'utf-8').replace(/\r\n/g, '\n');
	const body = md.split(/^## Overview\s*$/m)[1] ?? md;
	const para = body
		.split(/\n\s*\n/)
		.map((p) => p.trim())
		.find((p) => p && !p.startsWith('#') && !p.startsWith('!') && !p.startsWith('```'));
	if (!para) return '';
	const plain = para
		.replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
		.replace(/\*\*|`/g, '')
		.replace(/\s+/g, ' ');
	return firstSentence(plain);
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

		const code = fs.readFileSync(mainPath, 'utf-8').replace(/\r\n/g, '\n');
		const slug = entry.name;

		examples.push({
			slug,
			title: slugToTitle(slug),
			description: docComment(code) || readmeSummary(path.join(dir, entry.name)),
			category: categoryMap[slug] ?? (slug.startsWith('spike-') ? 'spikes' : 'other'),
			code
		});
	}

	const order = Object.keys(categoryMap);
	const rank = (slug: string) => {
		const i = order.indexOf(slug);
		return i === -1 ? 999 : i;
	};
	examples.sort((a, b) => rank(a.slug) - rank(b.slug) || a.slug.localeCompare(b.slug));

	return examples;
}

export function getCategoryLabels(): Record<string, string> {
	return categoryLabels;
}
