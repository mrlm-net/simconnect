// @ts-nocheck
import { visit } from 'unist-util-visit';

/**
 * Slugify text to create a GitHub-style heading ID.
 * @param {string} text
 * @returns {string}
 */
function slugify(text) {
	return text
		.toLowerCase()
		.replace(/[^\w\s-]/g, '')
		.replace(/\s+/g, '-')
		.replace(/-+/g, '-')
		.trim();
}

/**
 * Decode common HTML entities that mdsvex encodes in inline code spans.
 * @param {string} str
 * @returns {string}
 */
function decodeEntities(str) {
	return str
		.replace(/&amp;/g, '&')
		.replace(/&lt;/g, '<')
		.replace(/&gt;/g, '>')
		.replace(/&quot;/g, '"')
		.replace(/&#39;/g, "'");
}

/**
 * Extract text content from HAST node.
 * Decodes HTML entities so slugs match raw markdown heading text.
 * @param {import('unist').Node} node
 * @returns {string}
 */
function extractText(node) {
	if (node.type === 'text') {
		return decodeEntities(node.value ?? '');
	}
	if (node.children) {
		return node.children.map(extractText).join('');
	}
	return '';
}

/**
 * Rehype plugin that adds `id` attributes to h1-h6 elements.
 * Deduplicates IDs by appending -2, -3, ... for repeated slugs.
 */
export default function rehypeSlug() {
	return function transformer(tree) {
		const seen = new Map();
		visit(tree, 'element', (node) => {
			if (/^h[1-6]$/.test(node.tagName)) {
				if (!node.properties) {
					node.properties = {};
				}
				if (!node.properties.id) {
					const text = extractText(node);
					const base = slugify(text);
					const count = seen.get(base) ?? 0;
					seen.set(base, count + 1);
					node.properties.id = count === 0 ? base : `${base}-${count + 1}`;
				}
			}
		});
	};
}
