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
let pdfUploadedUrl = $state('');

// Readiness tracking
let textExtractionDone = $state(false);
let textExtractionFailed = $state(false);
let uploadDone = $state(false);
let uploadFailed = $state(false);

// OCR state
let ocrPageTexts = $state<Map<number, string>>(new Map());
let ocrCompletedPages = $state<Set<number>>(new Set());
let ocrInProgress = $state(false);
let ocrProgress = $state<{ done: number; total: number } | null>(null);
let ocrJobId = $state<string | null>(null);
let ocrPollTimer = $state<ReturnType<typeof setInterval> | null>(null);

// --- Annotation state ---
let annotationData = $state<Map<number, ImageData>>(new Map());
let isDrawing = $state(false);
let penColor = $state('#ff0000');
let penSize = $state(3);

// --- Extra documents (additional attachments beyond the PDF) ---
let extraDocuments = $state<{ file: File; name: string }[]>([]);

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

// --- Canvas references ---
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

export function getExtraDocuments() { return extraDocuments; }

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
export function getOcrCompletedPages() { return ocrCompletedPages; }
export function getIsOcrAvailable() { return isOcrEnabled(); }

// --- Readiness ---
export function isPdfReady(): boolean {
	return pdfFile !== null && textExtractionDone && uploadDone && !ocrInProgress;
}

export function getContextStatus(): { ready: boolean; label: string; detail: string } {
	if (!pdfFile) return { ready: false, label: 'No PDF', detail: '' };
	if (!textExtractionDone && !textExtractionFailed) return { ready: false, label: 'Extracting text...', detail: '' };
	if (!uploadDone && !uploadFailed) return { ready: false, label: 'Uploading PDF...', detail: '' };
	if (ocrInProgress) return { ready: false, label: 'OCR in progress...', detail: '' };

	if (textExtractionFailed && ocrCompletedPages.size === 0) {
		return { ready: false, label: 'Text extraction failed', detail: 'Use OCR to extract text from this PDF.' };
	}
	if (uploadFailed) {
		return { ready: false, label: 'Upload failed', detail: 'The PDF could not be uploaded. Try reloading it.' };
	}

	const effectiveTexts = pdfPageTexts.map((t, i) => ocrPageTexts.get(i) ?? t);
	const pagesWithText = effectiveTexts.filter(t => t.trim().length > 0).length;
	const ocrCount = ocrCompletedPages.size;

	let detail = `${pagesWithText}/${pdfPageCount} pages have text`;
	if (ocrCount > 0) detail += ` (${ocrCount} OCR'd)`;
	detail += ` · Viewing page ${currentPage}`;

	return { ready: true, label: 'Ready', detail };
}

// --- PDF loading ---
export async function loadPdf(file: File) {
	isProcessing = true;
	pdfChatError = null;
	pdfFilename = file.name;
	textExtractionDone = false;
	textExtractionFailed = false;
	uploadDone = false;
	uploadFailed = false;

	try {
		const count = await getPageCount(file);
		pdfPageCount = count;
		pdfFile = file;
		currentPage = 1;
		isProcessing = false;

		const [uploadResult, textResult] = await Promise.allSettled([
			api.uploadDocument(file),
			extractPageTexts(file)
		]);

		if (uploadResult.status === 'fulfilled') {
			pdfUploadedUrl = uploadResult.value.url;
			uploadDone = true;
		} else {
			uploadFailed = true;
		}

		if (textResult.status === 'fulfilled') {
			pdfPageTexts = textResult.value;
			textExtractionDone = true;
		} else {
			textExtractionFailed = true;
			pdfPageTexts = Array(count).fill('');
			textExtractionDone = true;
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

// --- Canvas refs ---
export function setCanvasRefs(pdf: HTMLCanvasElement | null, annot: HTMLCanvasElement | null) {
	pdfCanvasRef = pdf;
	annotCanvasRef = annot;
}

// --- Extra document uploads ---
export function addExtraDocument(file: File) {
	extraDocuments = [...extraDocuments, { file, name: file.name }];
}

export function removeExtraDocument(index: number) {
	extraDocuments = extraDocuments.filter((_, i) => i !== index);
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
		const ocrPages: { page_num: number; image_url: string }[] = [];
		for (const pageNum of pages) {
			const blob = await renderPageToBlob(pdfFile, pageNum);
			const imageFile = new File([blob], `page-${pageNum}.png`, { type: 'image/png' });
			const result = await api.uploadImage(imageFile);
			ocrPages.push({ page_num: pageNum, image_url: result.url });
		}

		const { job_id } = await api.startOCR(pdfUploadedUrl, pdfPageCount, ocrPages);
		ocrJobId = job_id;

		ocrPollTimer = setInterval(async () => {
			if (!ocrJobId) return;
			try {
				const status = await api.getOCRStatus(ocrJobId);
				ocrProgress = { done: status.done_pages, total: status.total_pages };

				if (status.status === 'complete' && status.result_text) {
					const resultPages = status.result_text.split('\n\n---\n\n');
					const newOcrMap = new Map(ocrPageTexts);
					const newTexts = [...pdfPageTexts];
					for (let i = 0; i < resultPages.length; i++) {
						const text = resultPages[i].replace(/^## Page \d+\n\n/, '');
						newOcrMap.set(i, text);
						if (i < newTexts.length) newTexts[i] = text;
					}
					ocrPageTexts = newOcrMap;
					pdfPageTexts = newTexts;

					const newCompleted = new Set(ocrCompletedPages);
					for (let i = 0; i < resultPages.length; i++) newCompleted.add(i + 1);
					ocrCompletedPages = newCompleted;
					textExtractionFailed = false;
					cleanupOcr();
				} else if (status.status === 'error') {
					pdfChatError = status.error || 'OCR failed.';
					cleanupOcr();
				} else if (status.status === 'cancelled') {
					cleanupOcr();
				}
			} catch { /* polling error, continue */ }
		}, 1000);
	} catch (err) {
		pdfChatError = err instanceof Error ? err.message : 'Failed to start OCR.';
		ocrInProgress = false;
		ocrProgress = null;
	}
}

export async function cancelPdfOcr() {
	if (ocrJobId) { try { await api.cancelOCR(ocrJobId); } catch { /* ignore */ } }
	cleanupOcr();
}

function cleanupOcr() {
	if (ocrPollTimer) { clearInterval(ocrPollTimer); ocrPollTimer = null; }
	ocrJobId = null;
	ocrInProgress = false;
	ocrProgress = null;
}

// --- Context building ---
export function getPdfContextText(isExternal: boolean): string {
	const texts = pdfPageTexts;
	if (texts.length === 0) return '';

	const effectiveTexts = texts.map((t, i) => ocrPageTexts.get(i) ?? t);

	if (isExternal) {
		return effectiveTexts.map((t, i) => `--- Page ${i + 1} ---\n${t}`).join('\n\n');
	}

	const start = Math.max(0, currentPage - 1 - 25);
	const end = Math.min(texts.length, currentPage - 1 + 26);
	return effectiveTexts.slice(start, end)
		.map((t, i) => `--- Page ${start + i + 1} ---\n${t}`).join('\n\n');
}

// --- Capture current page with annotations ---
async function captureCurrentPageWithAnnotations(): Promise<string> {
	if (!pdfCanvasRef) {
		throw new Error('Current page is not rendered. Scroll to make it visible and try again.');
	}

	const w = pdfCanvasRef.width;
	const h = pdfCanvasRef.height;
	const composite = document.createElement('canvas');
	composite.width = w;
	composite.height = h;
	const ctx = composite.getContext('2d')!;
	ctx.drawImage(pdfCanvasRef, 0, 0);
	if (annotCanvasRef) ctx.drawImage(annotCanvasRef, 0, 0);

	const blob = await new Promise<Blob | null>((resolve) => composite.toBlob(resolve, 'image/png'));
	if (!blob) throw new Error('Failed to capture page image.');

	const file = new File([blob], `page-${currentPage}.png`, { type: 'image/png' });
	const result = await api.uploadImage(file);
	return result.url;
}

// --- Chat ---
export async function sendPdfMessage(content: string) {
	if (!pdfFile || pdfIsStreaming) return;
	pdfChatError = null;

	// Pre-send validation
	if (!textExtractionDone) { pdfChatError = 'Text extraction is still in progress. Please wait.'; return; }
	if (uploadFailed) { pdfChatError = 'PDF upload failed. Reload the PDF and try again.'; return; }
	if (ocrInProgress) { pdfChatError = 'OCR is in progress. Wait for it to finish or cancel it first.'; return; }

	const modelId = getEffectiveModelId();
	if (!modelId) { pdfChatError = 'No model selected. Please select a model first.'; return; }
	const modelInfo = getModels().find((m) => m.id === modelId);
	const isExternal = modelInfo?.external ?? false;

	const effectiveTexts = pdfPageTexts.map((t, i) => ocrPageTexts.get(i) ?? t);
	const pagesWithText = effectiveTexts.filter(t => t.trim().length > 0).length;
	if (pagesWithText === 0) {
		pdfChatError = 'No text could be extracted from this PDF. Use OCR to extract text from scanned pages before chatting.';
		return;
	}

	// Build document attachments: PDF context + any extra documents
	const contextText = getPdfContextText(isExternal);
	const allDocs: DocumentAttachment[] = [
		{ filename: pdfFilename, url: pdfUploadedUrl || '', text: contextText }
	];

	// Upload and attach extra documents
	if (extraDocuments.length > 0) {
		try {
			for (const doc of extraDocuments) {
				const uploaded = await api.uploadDocument(doc.file);
				allDocs.push({
					filename: uploaded.original_name,
					url: uploaded.url,
					text: uploaded.extracted_text
				});
			}
		} catch (err) {
			pdfChatError = err instanceof Error ? err.message : 'Failed to upload additional document.';
			return;
		}
	}

	// Capture current page with annotations
	let pageImageUrl: string;
	try {
		pageImageUrl = await captureCurrentPageWithAnnotations();
	} catch (err) {
		pdfChatError = err instanceof Error ? err.message : 'Failed to capture current page.';
		return;
	}

	// All validated — add user message
	const userMsg: ChatMessage = { role: 'user', content };
	if (extraDocuments.length > 0) {
		userMsg.documents = allDocs.slice(1); // show extra docs on the message bubble
	}
	pdfMessages = [...pdfMessages, userMsg];

	// Clear extra documents after they've been sent
	extraDocuments = [];

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
			[pageImageUrl],
			allDocs,
			modelId,
			pdfAbortController.signal,
			{
				appType: !pdfConversationId ? 'pdf-view' : undefined,
				pdfView: { current_page: currentPage, total_pages: pdfPageCount }
			}
		)) {
			if (token.conversation_id && !pdfConversationId) {
				pdfConversationId = token.conversation_id;
			}
			if (token.queue_id) pdfQueueItemId = token.queue_id;
			if (token.position !== undefined) pdfQueuePosition = token.position;
			if (token.reasoning) pdfStreamingReasoning += token.reasoning;
			if (token.thinking_duration !== undefined) pdfThinkingDuration = token.thinking_duration;
			if (token.tool_status) pdfToolStatus = token.tool_status;
			if (token.content) {
				pdfQueuePosition = null;
				pdfToolStatus = null;
				pdfStreamingContent += token.content;
			}
			if (token.error) pdfChatError = token.error;
		}

		if (pdfStreamingContent) {
			const msg: ChatMessage = { role: 'assistant', content: pdfStreamingContent };
			if (pdfStreamingReasoning) msg.reasoning = pdfStreamingReasoning;
			if (pdfThinkingDuration !== null) msg.thinking_duration = pdfThinkingDuration;
			pdfMessages = [...pdfMessages, msg];
		}
		await loadConversations();
		if (isNewConversation) setTimeout(() => loadConversations(), 3000);
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
	if (pdfAbortController) { pdfAbortController.abort(); pdfAbortController = null; }
}

export async function cancelPdfQueueItem() {
	const id = pdfQueueItemId;
	if (id) { try { await api.cancelQueueItem(id); } catch { /* ignore */ } }
	cancelPdfStream();
}

export async function loadPdfConversation(convId: string) {
	try {
		const conv = await api.getConversation(convId);
		pdfConversationId = conv.id;
		pdfMessages = conv.messages;
	} catch (err) {
		pdfChatError = err instanceof Error ? err.message : 'Failed to load conversation.';
	}
}

export function resetPdfView() {
	pdfFile = null;
	pdfPageCount = 0;
	currentPage = 1;
	pdfPageTexts = [];
	pdfFilename = '';
	isProcessing = false;
	pdfUploadedUrl = '';
	textExtractionDone = false;
	textExtractionFailed = false;
	uploadDone = false;
	uploadFailed = false;
	ocrPageTexts = new Map();
	ocrCompletedPages = new Set();
	cleanupOcr();
	extraDocuments = [];

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
