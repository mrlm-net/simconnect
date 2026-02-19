// @ts-nocheck
import { visit } from 'unist-util-visit';
import hljs from 'highlight.js/lib/core';
import go from 'highlight.js/lib/languages/go';
import bash from 'highlight.js/lib/languages/bash';
import json from 'highlight.js/lib/languages/json';
import yaml from 'highlight.js/lib/languages/yaml';
import javascript from 'highlight.js/lib/languages/javascript';
import typescript from 'highlight.js/lib/languages/typescript';

hljs.registerLanguage('go', go);
hljs.registerLanguage('bash', bash);
hljs.registerLanguage('json', json);
hljs.registerLanguage('yaml', yaml);
hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('typescript', typescript);

/**
 * Extract text content from a HAST node tree.
 * @param {import('unist').Node} node
 * @returns {string}
 */
function extractText(node) {
	if (node.type === 'text') {
		return node.value ?? '';
	}
	if (node.children) {
		return node.children.map(extractText).join('');
	}
	return '';
}

/**
 * Parse highlight.js HTML output into HAST nodes.
 * Simple parser that handles <span class="...">...</span> and text.
 * @param {string} html
 * @returns {Array<import('hast').Element | import('hast').Text>}
 */
function parseHljsHtml(html) {
	/** @type {Array<any>} */
	const nodes = [];
	let pos = 0;

	while (pos < html.length) {
		const tagStart = html.indexOf('<', pos);

		if (tagStart === -1) {
			// Rest is text
			const text = html.slice(pos);
			if (text) {
				nodes.push({ type: 'text', value: decodeEntities(text) });
			}
			break;
		}

		// Text before the tag
		if (tagStart > pos) {
			const text = html.slice(pos, tagStart);
			if (text) {
				nodes.push({ type: 'text', value: decodeEntities(text) });
			}
		}

		// Check for closing tag
		if (html.startsWith('</span>', tagStart)) {
			pos = tagStart + 7;
			// Return current level — caller handles nesting
			break;
		}

		// Opening <span ...>
		const spanMatch = html.slice(tagStart).match(/^<span class="([^"]*)">/);
		if (spanMatch) {
			pos = tagStart + spanMatch[0].length;

			// Recursively parse children until </span>
			const children = [];
			while (pos < html.length && !html.startsWith('</span>', pos)) {
				const nextTag = html.indexOf('<', pos);
				if (nextTag === -1) {
					children.push({ type: 'text', value: decodeEntities(html.slice(pos)) });
					pos = html.length;
					break;
				}
				if (nextTag > pos) {
					children.push({ type: 'text', value: decodeEntities(html.slice(pos, nextTag)) });
					pos = nextTag;
				}
				if (html.startsWith('</span>', pos)) {
					break;
				}
				// Nested span
				const nestedMatch = html.slice(pos).match(/^<span class="([^"]*)">/);
				if (nestedMatch) {
					pos += nestedMatch[0].length;
					const nestedChildren = [];
					while (pos < html.length && !html.startsWith('</span>', pos)) {
						const nt = html.indexOf('<', pos);
						if (nt === -1) {
							nestedChildren.push({ type: 'text', value: decodeEntities(html.slice(pos)) });
							pos = html.length;
							break;
						}
						if (nt > pos) {
							nestedChildren.push({
								type: 'text',
								value: decodeEntities(html.slice(pos, nt))
							});
						}
						if (html.startsWith('</span>', nt)) {
							pos = nt;
							break;
						}
						// Skip unknown tags
						pos = nt + 1;
					}
					if (html.startsWith('</span>', pos)) {
						pos += 7;
					}
					children.push({
						type: 'element',
						tagName: 'span',
						properties: { className: nestedMatch[1].split(' ') },
						children: nestedChildren
					});
				} else {
					// Unknown tag, skip char
					pos += 1;
				}
			}
			if (html.startsWith('</span>', pos)) {
				pos += 7;
			}

			nodes.push({
				type: 'element',
				tagName: 'span',
				properties: { className: spanMatch[1].split(' ') },
				children
			});
		} else {
			// Not a span, output char and advance
			nodes.push({ type: 'text', value: '<' });
			pos = tagStart + 1;
		}
	}

	return nodes;
}

/**
 * Decode common HTML entities.
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
 * Rehype plugin for syntax highlighting with highlight.js.
 * Only highlights code blocks with an explicit language class.
 */
export default function rehypeHighlight() {
	return function transformer(tree) {
		visit(tree, 'element', (node) => {
			if (node.tagName !== 'pre') return;
			if (!node.children || node.children.length === 0) return;

			const codeNode = node.children.find(
				(/** @type {any} */ child) => child.type === 'element' && child.tagName === 'code'
			);
			if (!codeNode) return;

			const className = codeNode.properties?.className;
			if (!className || !Array.isArray(className)) return;

			const langClass = className.find(
				(/** @type {string} */ c) => typeof c === 'string' && c.startsWith('language-')
			);
			if (!langClass) return;

			const language = langClass.replace('language-', '');
			if (!hljs.getLanguage(language)) return;

			const text = extractText(codeNode);
			const result = hljs.highlight(text, { language });

			codeNode.children = parseHljsHtml(result.value);
			if (!codeNode.properties.className.includes('hljs')) {
				codeNode.properties.className.push('hljs');
			}
		});
	};
}
