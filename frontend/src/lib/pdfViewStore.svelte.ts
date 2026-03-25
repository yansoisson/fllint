import * as api from './api';
import { getPageCount, extractPageTexts, renderPageToBlob, parsePageRange } from './pdf';
import { getEffectiveModelId, getModels, loadConversations, isOcrEnabled } from './stores.svelte';
import type { ChatMessage, DocumentAttachment } from './types';

// --- PDF state ---
let pdfFile = $state<File | null>(null);
let pdfPageCount = $state(0);
let currentPage = $state(1);
let pdfPageTexts = $state<string[]>([]);
let isProcessing = $state(false);
let pdfFilename = $state('');
let pdfUploadedUrl = $state(''); // URL from /api/document/upload

// OCR state
let ocrPageTexts = $state<Map<number, string>>(new Map());
let ocrInProgress = $state(false);
let ocrProgress = $state<{ done: number; total: number } | null>(null);
let ocrJobId = $state<string | null>(null);
let ocrPollTimer = $state<ReturnType<typeof setInterval> | null>(null);

// --- Annotation state ---
let annotationData = $state<Map<number, ImageData>>(new Map());
let isDrawing = $state(false);
let penColor = $state('#ff0000');
let penSize = $state(3);

// --- Chat state ---
let pdfMessages = $state<ChatMessage[]>([]);
let pdfConversationId = $state<string | null>(null);
let pdfIsStreaming = $state(false);
let pdfStreamingContent = $state('');
let pdfStreamingReasoning = $state('');
let pdfThinkingDuration = $state<number | null>(null);
let pdfChatError = $state<string | null>(null);
let pdfAbortController = $state<AbortController | null>(null);
let pdfQueuePosition = $state<number | null>(null);
let pdfQueueItemId = $state<string | null>(null);
let pdfToolStatus = $state<string | null>(null);

// --- Canvas references (set by PdfViewer) ---
let pdfCanvasRef = $state<HTMLCanvasElement | null>(null);
let annotCanvasRef = $state<HTMLCanvasElement | null>(null);

// --- Getters ---
export function getPdfFile() { return pdfFile; }
export function getPdfPageCount() { return pdfPageCount; }
export function getCurrentPage() { return currentPage; }
export function getPdfPageTexts() { return pdfPageTexts; }
export function getIsProcessing() { return isProcessing; }
export function getPdfFilename() { return pdfFilename; }
export function getOcrPageTexts() { return ocrPageTexts; }

export function getAnnotationData() { return annotationData; }
export function getIsDrawing() { return isDrawing; }
export function getPenColor() { return penColor; }
export function getPenSize() { return penSize; }

export function getPdfMessages() { return pdfMessages; }
export function getPdfConversationId() { return pdfConversationId; }
export function getPdfIsStreaming() { return pdfIsStreaming; }
export function getPdfStreamingContent() { return pdfStreamingContent; }
export function getPdfStreamingReasoning() { return pdfStreamingReasoning; }
export function getPdfThinkingDuration() { return pdfThinkingDuration; }
export function getPdfChatError() { return pdfChatError; }
export function getPdfQueuePosition() { return pdfQueuePosition; }
export function getPdfToolStatus() { return pdfToolStatus; }

export function getOcrInProgress() { return ocrInProgress; }
export function getOcrProgress() { return ocrProgress; }
export function getIsOcrAvailable() { return isOcrEnabled(); }

// --- PDF loading (two-phase: page count first, then text extraction + upload) ---
export async function loadPdf(file: File) {
	isProcessing = true;
	pdfChatError = null;
	pdfFilename = file.name;

	try {
		// Phase 1: Get page count (fast) — then show the viewer immediately
		const count = await getPageCount(file);
		pdfPageCount = count;
		pdfFile = file;
		currentPage = 1;
		isProcessing = false;

		// Phase 2: Upload PDF to backend + extract text (parallel, non-blocking for UI)
		const [uploadResult] = await Promise.allSettled([
			api.uploadDocument(file),
			extractPageTexts(file).then(texts => { pdfPageTexts = texts; })
		]);

		if (uploadResult.status === 'fulfilled') {
			pdfUploadedUrl = uploadResult.value.url;
		} else {
			console.warn('PDF upload failed:', uploadResult.reason);
		}

		// If text extraction failed, fill with empty strings
		if (pdfPageTexts.length === 0) {
			pdfPageTexts = Array(count).fill('');
		}
	} catch (err) {
		pdfChatError = err instanceof Error ? err.message : 'Failed to open PDF.';
		pdfFile = null;
		pdfFilename = '';
		pdfPageCount = 0;
		isProcessing = false;
	}
}

// --- Page navigation ---
export function setCurrentPage(page: number) {
	if (page >= 1 && page <= pdfPageCount) {
		currentPage = page;
	}
}

// --- Annotation controls ---
export function setIsDrawing(val: boolean) { isDrawing = val; }
export function setPenColor(color: string) { penColor = color; }
export function setPenSize(size: number) { penSize = size; }

export function saveAnnotation(page: number, data: ImageData) {
	const next = new Map(annotationData);
	next.set(page, data);
	annotationData = next;
}

export function clearAnnotation(page: number) {
	const next = new Map(annotationData);
	next.delete(page);
	annotationData = next;
}

export function clearAllAnnotations() {
	annotationData = new Map();
}

// --- Canvas refs (set by PdfViewer for current page) ---
export function setCanvasRefs(pdf: HTMLCanvasElement | null, annot: HTMLCanvasElement | null) {
	pdfCanvasRef = pdf;
	annotCanvasRef = annot;
}

// --- OCR ---
export async function startPdfOcr(pageRangeStr: string) {
	if (!pdfFile || ocrInProgress) return;
	if (!pdfUploadedUrl) {
		pdfChatError = 'PDF not uploaded yet. Please wait and try again.';
		return;
	}

	const pages = parsePageRange(pageRangeStr, pdfPageCount);
	if (pages.length === 0) return;

	ocrInProgress = true;
	ocrProgress = { done: 0, total: pages.length };
	pdfChatError = null;

	try {
		// Render selected pages to images and upload them
		const ocrPages: { page_num: number; image_url: string }[] = [];
		for (const pageNum of pages) {
			const blob = await renderPageToBlob(pdfFile, pageNum);
			const imageFile = new File([blob], `page-${pageNum}.png`, { type: 'image/png' });
			const result = await api.uploadImage(imageFile);
			ocrPages.push({ page_num: pageNum, image_url: result.url });
		}

		// Start OCR
		const { job_id } = await api.startOCR(pdfUploadedUrl, pdfPageCount, ocrPages);
		ocrJobId = job_id;

		// Poll for progress
		ocrPollTimer = setInterval(async () => {
			if (!ocrJobId) return;
			try {
				const status = await api.getOCRStatus(ocrJobId);
				ocrProgress = { done: status.done_pages, total: status.total_pages };

				if (status.status === 'complete' && status.result_text) {
					// Parse the result text back into per-page texts
					const resultPages = status.result_text.split('\n\n---\n\n');
					for (let i = 0; i < resultPages.length; i++) {
						const text = resultPages[i].replace(/^## Page \d+\n\n/, '');
						ocrPageTexts = new Map(ocrPageTexts);
						ocrPageTexts.set(i, text);
						// Also update pdfPageTexts for pages that were OCR'd
						if (i < pdfPageTexts.length) {
							pdfPageTexts = [...pdfPageTexts];
							pdfPageTexts[i] = text;
						}
					}
					cleanupOcr();
				} else if (status.status === 'error') {
					pdfChatError = status.error || 'OCR failed.';
					cleanupOcr();
				} else if (status.status === 'cancelled') {
					cleanupOcr();
				}
			} catch {
				// Polling error, continue
			}
		}, 1000);
	} catch (err) {
		pdfChatError = err instanceof Error ? err.message : 'Failed to start OCR.';
		ocrInProgress = false;
		ocrProgress = null;
	}
}

export async function cancelPdfOcr() {
	if (ocrJobId) {
		try {
			await api.cancelOCR(ocrJobId);
		} catch { /* ignore */ }
	}
	cleanupOcr();
}

function cleanupOcr() {
	if (ocrPollTimer) {
		clearInterval(ocrPollTimer);
		ocrPollTimer = null;
	}
	ocrJobId = null;
	ocrInProgress = false;
	ocrProgress = null;
}

// --- Context building ---
export function getPdfContextText(isExternal: boolean): string {
	const texts = pdfPageTexts;
	if (texts.length === 0) return '';

	// Build per-page text, preferring OCR text where available
	const effectiveTexts = texts.map((t, i) => ocrPageTexts.get(i) ?? t);

	if (isExternal) {
		// External models: send full PDF text
		return effectiveTexts
			.map((t, i) => `--- Page ${i + 1} ---\n${t}`)
			.join('\n\n');
	}

	// Local models: 25 pages before and after current page
	const start = Math.max(0, currentPage - 1 - 25);
	const end = Math.min(texts.length, currentPage - 1 + 26);
	const slice = effectiveTexts.slice(start, end);
	return slice
		.map((t, i) => `--- Page ${start + i + 1} ---\n${t}`)
		.join('\n\n');
}

// --- Capture current page with annotations ---
export async function captureCurrentPageWithAnnotations(): Promise<string | null> {
	if (!pdfCanvasRef) return null;

	const w = pdfCanvasRef.width;
	const h = pdfCanvasRef.height;

	const composite = document.createElement('canvas');
	composite.width = w;
	composite.height = h;
	const ctx = composite.getContext('2d')!;
	ctx.drawImage(pdfCanvasRef, 0, 0);
	if (annotCanvasRef) {
		ctx.drawImage(annotCanvasRef, 0, 0);
	}

	const blob = await new Promise<Blob | null>((resolve) => composite.toBlob(resolve, 'image/png'));
	if (!blob) return null;

	try {
		const file = new File([blob], `page-${currentPage}.png`, { type: 'image/png' });
		const result = await api.uploadImage(file);
		return result.url;
	} catch {
		return null;
	}
}

// --- Chat ---
export async function sendPdfMessage(content: string) {
	if (!pdfFile || pdfIsStreaming) return;
	pdfChatError = null;

	const modelId = getEffectiveModelId();
	const modelInfo = getModels().find((m) => m.id === modelId);
	const isExternal = modelInfo?.external ?? false;

	// Build document context with uploaded URL
	const contextText = getPdfContextText(isExternal);
	const docAttachment: DocumentAttachment = {
		filename: pdfFilename,
		url: pdfUploadedUrl || '',
		text: contextText
	};

	// Capture current page image with annotations
	const pageImageUrl = await captureCurrentPageWithAnnotations();

	// Add user message to local display
	const userMsg: ChatMessage = { role: 'user', content };
	pdfMessages = [...pdfMessages, userMsg];

	pdfIsStreaming = true;
	pdfStreamingContent = '';
	pdfStreamingReasoning = '';
	pdfThinkingDuration = null;
	pdfAbortController = new AbortController();
	pdfQueuePosition = null;
	pdfQueueItemId = null;
	pdfToolStatus = null;

	const isNewConversation = !pdfConversationId;

	try {
		for await (const token of api.streamChat(
			content,
			pdfConversationId ?? undefined,
			pageImageUrl ? [pageImageUrl] : undefined,
			[docAttachment],
			modelId ?? undefined,
			pdfAbortController.signal,
			{ appType: !pdfConversationId ? 'pdf-view' : undefined }
		)) {
			if (token.conversation_id && !pdfConversationId) {
				pdfConversationId = token.conversation_id;
			}
			if (token.queue_id) {
				pdfQueueItemId = token.queue_id;
			}
			if (token.position !== undefined) {
				pdfQueuePosition = token.position;
			}
			if (token.reasoning) {
				pdfStreamingReasoning += token.reasoning;
			}
			if (token.thinking_duration !== undefined) {
				pdfThinkingDuration = token.thinking_duration;
			}
			if (token.tool_status) {
				pdfToolStatus = token.tool_status;
			}
			if (token.content) {
				pdfQueuePosition = null;
				pdfToolStatus = null;
				pdfStreamingContent += token.content;
			}
			if (token.error) {
				pdfChatError = token.error;
			}
		}

		if (pdfStreamingContent) {
			const msg: ChatMessage = { role: 'assistant', content: pdfStreamingContent };
			if (pdfStreamingReasoning) {
				msg.reasoning = pdfStreamingReasoning;
			}
			if (pdfThinkingDuration !== null) {
				msg.thinking_duration = pdfThinkingDuration;
			}
			pdfMessages = [...pdfMessages, msg];
		}
		await loadConversations();
		if (isNewConversation) {
			setTimeout(() => loadConversations(), 3000);
		}
	} catch (err) {
		if (err instanceof DOMException && err.name === 'AbortError') {
			if (pdfStreamingContent) {
				const msg: ChatMessage = { role: 'assistant', content: pdfStreamingContent };
				if (pdfStreamingReasoning) msg.reasoning = pdfStreamingReasoning;
				if (pdfThinkingDuration !== null) msg.thinking_duration = pdfThinkingDuration;
				pdfMessages = [...pdfMessages, msg];
			}
		} else {
			pdfChatError = err instanceof Error ? err.message : 'Failed to get response.';
		}
	} finally {
		pdfIsStreaming = false;
		pdfStreamingContent = '';
		pdfStreamingReasoning = '';
		pdfThinkingDuration = null;
		pdfToolStatus = null;
		pdfAbortController = null;
		pdfQueuePosition = null;
		pdfQueueItemId = null;
	}
}

export function cancelPdfStream() {
	if (pdfAbortController) {
		pdfAbortController.abort();
		pdfAbortController = null;
	}
}

export async function cancelPdfQueueItem() {
	const id = pdfQueueItemId;
	if (id) {
		try {
			await api.cancelQueueItem(id);
		} catch (err) {
			console.error('Failed to cancel queue item:', err);
		}
	}
	cancelPdfStream();
}

// --- Load existing PDF conversation ---
export async function loadPdfConversation(convId: string) {
	try {
		const conv = await api.getConversation(convId);
		pdfConversationId = conv.id;
		pdfMessages = conv.messages;
	} catch (err) {
		pdfChatError = err instanceof Error ? err.message : 'Failed to load conversation.';
	}
}

// --- Reset ---
export function resetPdfView() {
	pdfFile = null;
	pdfPageCount = 0;
	currentPage = 1;
	pdfPageTexts = [];
	pdfFilename = '';
	isProcessing = false;
	pdfUploadedUrl = '';
	ocrPageTexts = new Map();
	cleanupOcr();

	annotationData = new Map();
	isDrawing = false;
	penColor = '#ff0000';
	penSize = 3;

	pdfMessages = [];
	pdfConversationId = null;
	pdfIsStreaming = false;
	pdfStreamingContent = '';
	pdfStreamingReasoning = '';
	pdfThinkingDuration = null;
	pdfChatError = null;
	pdfAbortController = null;
	pdfQueuePosition = null;
	pdfQueueItemId = null;
	pdfToolStatus = null;

	pdfCanvasRef = null;
	annotCanvasRef = null;
}
