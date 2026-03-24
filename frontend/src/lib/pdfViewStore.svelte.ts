import * as api from './api';
import { extractPageTexts } from './pdf';
import { getEffectiveModelId, getModels, loadConversations } from './stores.svelte';
import type { ChatMessage, DocumentAttachment } from './types';

// --- PDF state ---
let pdfFile = $state<File | null>(null);
let pdfPageCount = $state(0);
let currentPage = $state(1);
let pdfPageTexts = $state<string[]>([]);
let isProcessing = $state(false);
let pdfFilename = $state('');

// OCR text overrides per page (0-indexed)
let ocrPageTexts = $state<Map<number, string>>(new Map());

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

// --- PDF loading ---
export async function loadPdf(file: File) {
	isProcessing = true;
	pdfChatError = null;
	pdfFile = file;
	pdfFilename = file.name;

	try {
		const texts = await extractPageTexts(file);
		pdfPageTexts = texts;
		pdfPageCount = texts.length;
		currentPage = 1;
	} catch (err) {
		pdfChatError = err instanceof Error ? err.message : 'Failed to extract PDF text.';
		pdfFile = null;
		pdfFilename = '';
	} finally {
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

	// Build document context
	const contextText = getPdfContextText(isExternal);
	const docAttachment: DocumentAttachment = {
		filename: pdfFilename,
		url: '',
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
	ocrPageTexts = new Map();

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
