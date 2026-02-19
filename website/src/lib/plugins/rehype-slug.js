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
 */
export default function rehypeSlug() {
	return function transformer(tree) {
		visit(tree, 'element', (node) => {
			if (/^h[1-6]$/.test(node.tagName)) {
				if (!node.properties) {
					node.properties = {};
				}
				if (!node.properties.id) {
					const text = extractText(node);
					node.properties.id = slugify(text);
				}
			}
		});
	};
}
