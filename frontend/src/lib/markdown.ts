import { Marked } from 'marked';
import { markedHighlight } from 'marked-highlight';
import markedKatex from 'marked-katex-extension';
import hljs from 'highlight.js/lib/common';

// CSS is imported in +layout.svelte to avoid dev-mode ordering issues

const marked = new Marked(
	markedHighlight({
		langPrefix: 'hljs language-',
		highlight(code, lang) {
			if (lang && hljs.getLanguage(lang)) {
				return hljs.highlight(code, { language: lang }).value;
			}
			return hljs.highlightAuto(code).value;
		}
	}),
	markedKatex({
		throwOnError: false,
		output: 'html',
		nonStandard: true
	}),
	{
		gfm: true,
		breaks: true,
		async: false,
		renderer: {
			// Escape raw HTML from LLM output to prevent XSS
			html({ text }: { text: string }) {
				return text
					.replace(/&/g, '&amp;')
					.replace(/</g, '&lt;')
					.replace(/>/g, '&gt;');
			},
			// Open links in system browser
			link({ href, title, tokens }: { href: string; title?: string | null; tokens: any[] }) {
				const text = this.parser.parseInline(tokens);
				const t = title ? ` title="${title}"` : '';
				return `<a href="${href}"${t} target="_blank" rel="noopener noreferrer">${text}</a>`;
			}
		}
	}
);

/**
 * Close unclosed constructs so partial streaming content renders correctly.
 */
function prepareForStreaming(text: string): string {
	// Count triple-backtick fences (at line start)
	const fences = text.match(/^```/gm);
	if (fences && fences.length % 2 !== 0) {
		text += '\n```';
	}

	// Count $$ delimiters for display math
	const displayMath = text.match(/\$\$/g);
	if (displayMath && displayMath.length % 2 !== 0) {
		text += '$$';
	}

	return text;
}

export function renderMarkdown(text: string): string {
	const prepared = prepareForStreaming(text);
	return marked.parse(prepared, { async: false }) as string;
}
