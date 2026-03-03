function esc(s: string): string {
	return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function inlineFmt(text: string): string {
	// Inline code
	text = text.replace(/`([^`]+)`/g, '<code>$1</code>');
	// Bold
	text = text.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
	// Italic (not * followed by space, which is a bullet)
	text = text.replace(/(?<!\*)\*(?!\*|\s)(.+?)(?<!\*)\*(?!\*)/g, '<em>$1</em>');
	return text;
}

export function renderMarkdown(text: string): string {
	// Extract code blocks first to protect them
	const codeBlocks: string[] = [];
	let src = text.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, _lang, code) => {
		codeBlocks.push(`<pre><code>${esc(code.trim())}</code></pre>`);
		return `\x00CB${codeBlocks.length - 1}\x00`;
	});

	// Escape HTML in remaining text
	src = esc(src);

	// Restore code blocks (already escaped internally)
	src = src.replace(/\x00CB(\d+)\x00/g, (_, i) => codeBlocks[+i]);

	// Split into blocks by blank lines
	const blocks = src.split(/\n{2,}/);

	return blocks
		.map((block) => {
			if (block.startsWith('<pre>')) return block;

			const lines = block.split('\n');
			const nonEmpty = lines.filter((l) => l.trim());

			// Unordered list
			if (nonEmpty.length > 0 && nonEmpty.every((l) => /^[*\-•]\s/.test(l))) {
				return (
					'<ul>' +
					nonEmpty
						.map((l) => `<li>${inlineFmt(l.replace(/^[*\-•]\s+/, ''))}</li>`)
						.join('') +
					'</ul>'
				);
			}

			// Ordered list
			if (nonEmpty.length > 0 && nonEmpty.every((l) => /^\d+[.)]\s/.test(l))) {
				return (
					'<ol>' +
					nonEmpty
						.map((l) => `<li>${inlineFmt(l.replace(/^\d+[.)]\s+/, ''))}</li>`)
						.join('') +
					'</ol>'
				);
			}

			// Heading
			const hm = block.match(/^(#{1,6})\s+(.+)/);
			if (hm) return `<h${hm[1].length}>${inlineFmt(hm[2])}</h${hm[1].length}>`;

			// Paragraph
			return `<p>${inlineFmt(block).replace(/\n/g, '<br>')}</p>`;
		})
		.join('');
}
