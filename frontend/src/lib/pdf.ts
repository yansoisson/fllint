import * as pdfjs from 'pdfjs-dist';

// Configure the worker using Vite's import.meta.url
pdfjs.GlobalWorkerOptions.workerSrc = new URL(
	'pdfjs-dist/build/pdf.worker.min.mjs',
	import.meta.url
).toString();

/**
 * Get the number of pages in a PDF file.
 */
export async function getPageCount(file: File): Promise<number> {
	const arrayBuffer = await file.arrayBuffer();
	const pdf = await pdfjs.getDocument({ data: new Uint8Array(arrayBuffer) }).promise;
	const count = pdf.numPages;
	pdf.destroy();
	return count;
}

/**
 * Render a single PDF page to a PNG blob.
 * @param file The PDF file
 * @param pageNum 1-based page number
 * @param scale Render scale (2.0 = 2x resolution for better OCR)
 */
export async function renderPageToBlob(file: File, pageNum: number, scale: number = 1.5): Promise<Blob> {
	const arrayBuffer = await file.arrayBuffer();
	const pdf = await pdfjs.getDocument({ data: new Uint8Array(arrayBuffer) }).promise;
	const page = await pdf.getPage(pageNum);
	const viewport = page.getViewport({ scale });

	const canvas = document.createElement('canvas');
	canvas.width = viewport.width;
	canvas.height = viewport.height;

	await page.render({
		canvasContext: canvas.getContext('2d')!,
		canvas,
		viewport
	}).promise;

	pdf.destroy();

	return new Promise((resolve, reject) => {
		canvas.toBlob((blob) => {
			if (blob) resolve(blob);
			else reject(new Error('Failed to render PDF page'));
		}, 'image/png');
	});
}

/**
 * Extract text content from each page of a PDF using pdf.js text layer.
 * Returns an array of strings, one per page (0-indexed: texts[0] = page 1).
 */
export async function extractPageTexts(file: File): Promise<string[]> {
	const arrayBuffer = await file.arrayBuffer();
	const pdf = await pdfjs.getDocument({ data: new Uint8Array(arrayBuffer) }).promise;
	const texts: string[] = [];
	for (let i = 1; i <= pdf.numPages; i++) {
		const page = await pdf.getPage(i);
		const content = await page.getTextContent();
		const items = content.items as Array<{ str: string }>;
		texts.push(items.map((item) => item.str).join(' '));
	}
	pdf.destroy();
	return texts;
}

/**
 * Parse a page range string like "1-3, 5, 7-10" into an array of page numbers.
 * Returns sorted, unique page numbers within [1, maxPages].
 */
export function parsePageRange(input: string, maxPages: number): number[] {
	const pages = new Set<number>();
	const parts = input.split(',').map((s) => s.trim()).filter(Boolean);

	for (const part of parts) {
		if (part.toLowerCase() === 'all') {
			for (let i = 1; i <= maxPages; i++) pages.add(i);
			continue;
		}

		const range = part.split('-').map((s) => parseInt(s.trim(), 10));
		if (range.length === 1 && !isNaN(range[0])) {
			if (range[0] >= 1 && range[0] <= maxPages) pages.add(range[0]);
		} else if (range.length === 2 && !isNaN(range[0]) && !isNaN(range[1])) {
			const start = Math.max(1, range[0]);
			const end = Math.min(maxPages, range[1]);
			for (let i = start; i <= end; i++) pages.add(i);
		}
	}

	return [...pages].sort((a, b) => a - b);
}
