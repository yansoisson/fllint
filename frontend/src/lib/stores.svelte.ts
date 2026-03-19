import type { Conversation, ChatMessage, ModelInfo, EngineStatus, MemoryInfo, MemoryErrorInfo, RegistryModel, DownloadStatus, Provider, DocumentAttachment, OcrJobStatus } from './types';
import { goto } from '$app/navigation';
import * as api from './api';
import { InsufficientMemoryError } from './api';

// --- Conversations ---
let conversations = $state<Conversation[]>([]);
let activeConversationId = $state<string | null>(null);
let messages = $state<ChatMessage[]>([]);

// --- Streaming ---
let isStreaming = $state(false);
let streamingContent = $state('');
let streamingReasoning = $state('');
let thinkingDuration = $state<number | null>(null);
let streamAbortController: AbortController | null = null;
let answerNowRequested = false;
let lastSentContent = '';
let lastSentImages: string[] = [];
let lastSentDocuments: DocumentAttachment[] = [];

// --- Queue ---
let queuePosition = $state<number | null>(null);
let queueItemId = $state<string | null>(null);

// --- Models ---
let models = $state<ModelInfo[]>([]);
let tabModelId = $state<string | null>(null); // per-tab model override (ephemeral)
let conversationModelId = $state<string | null>(null); // model bound to current conversation

// --- Engine Status ---
let engineStatus = $state<EngineStatus | null>(null);
let statusPollTimer: ReturnType<typeof setInterval> | null = null;

// --- Memory ---
let memoryInfo = $state<MemoryInfo | null>(null);

// --- Theme ---
let currentTheme = $state<'light' | 'dark' | 'system'>('system');

// --- Pro Mode ---
let proMode = $state(false);

// --- Pinned Models (from config) ---
let pinnedModelIds = $state<string[]>([]);

// --- Unload Popup (Pro Mode) ---
let unloadPopup = $state<{
	targetModelId: string;
	memoryError: MemoryErrorInfo;
} | null>(null);

// --- Version ---
let appVersion = $state<string | null>(null);

// --- UI ---
let sidebarOpen = $state(true);
let settingsOpen = $state(false);
let pendingImages = $state<{ file: File; preview: string }[]>([]);
let pendingDocuments = $state<{ file: File; name: string; ocrText?: string; ocrProcessing?: boolean }[]>([]);
let chatError = $state<string | null>(null);

// --- Draft Cache (restore input on send failure) ---
let draftText = $state('');
let draftImages = $state<{ file: File }[]>([]);
let draftDocuments = $state<{ file: File; name: string; ocrText?: string }[]>([]);
let sendFailed = $state<'pre-stream' | 'stream' | null>(null);

// --- OCR ---
let ocrEnabled = $state(false); // whether an OCR model is configured
let ocrPopup = $state<{ docIndex: number; filename: string; file: File; pageCount: number } | null>(null);
let ocrJobId = $state<string | null>(null);
let ocrProgress = $state<OcrJobStatus | null>(null);
let ocrPollTimer: ReturnType<typeof setInterval> | null = null;
let initError = $state<string | null>(null);
let notification = $state<{ message: string; type: 'error' | 'info' } | null>(null);
let notificationTimeout: ReturnType<typeof setTimeout> | null = null;

// --- Downloads ---
let downloadRegistry = $state<RegistryModel[]>([]);
let activeDownloads = $state<DownloadStatus[]>([]);
let downloadPollTimer: ReturnType<typeof setInterval> | null = null;

// --- Providers ---
let providers = $state<Provider[]>([]);

// --- Context Usage ---
let contextUsage = $state<{
	promptTokens: number;
	completionTokens: number;
	contextSize: number;
	finishReason: string;
} | null>(null);
let responseBuffer = $state(2048);
let lastResponseTruncated = $state(false);

// --- Settings Navigation ---
type SettingsTab = 'general' | 'models' | 'helper' | 'advanced';
let settingsInitialTab = $state<SettingsTab>('general');

// --- Cross-tab sync ---
let configChannel: BroadcastChannel | null = null;

// --- Getters ---
export function getConversations() {
	return conversations;
}
export function getActiveConversationId() {
	return activeConversationId;
}
export function getMessages() {
	return messages;
}
export function getIsStreaming() {
	return isStreaming;
}
export function getStreamingContent() {
	return streamingContent;
}
export function getStreamingReasoning() {
	return streamingReasoning;
}
export function getThinkingDuration() {
	return thinkingDuration;
}
export function getModels() {
	return models;
}
export function getActiveModel() {
	return models.find((m) => m.active) ?? null;
}
export function getTabModelId() {
	return tabModelId;
}
export function getConversationModelId() {
	return conversationModelId;
}
/** Returns the effective model ID for the current tab: explicit override > conversation binding > default */
export function getEffectiveModelId(): string | null {
	return tabModelId ?? conversationModelId ?? (getActiveModel()?.id ?? null);
}
export function getQueuePosition() {
	return queuePosition;
}
export function getQueueItemId() {
	return queueItemId;
}
export function getSidebarOpen() {
	return sidebarOpen;
}
export function getSettingsOpen() {
	return settingsOpen;
}
export function getPendingImages() {
	return pendingImages;
}
export function getEngineStatus() {
	return engineStatus;
}
export function getChatError() {
	return chatError;
}
export function getInitError() {
	return initError;
}
export function getNotification() {
	return notification;
}
export function getTheme() {
	return currentTheme;
}
export function getMemoryInfo() {
	return memoryInfo;
}
export function getUnloadPopup() {
	return unloadPopup;
}
export function getProMode() {
	return proMode;
}
export function getAppVersion() {
	return appVersion;
}
export function getPinnedModelIds() {
	return pinnedModelIds;
}
export function getDownloadRegistry() {
	return downloadRegistry;
}
export function getActiveDownloadsState() {
	return activeDownloads;
}
export function isDownloadActive(): boolean {
	return activeDownloads.some((d) => d.state === 'downloading' || d.state === 'queued');
}
export function getSettingsInitialTab() {
	return settingsInitialTab;
}
export function getProviders() {
	return providers;
}
export function getContextUsage() {
	return contextUsage;
}
export function getResponseBuffer() {
	return responseBuffer;
}
export function getLastResponseTruncated() {
	return lastResponseTruncated;
}
export function getSendFailed() {
	return sendFailed;
}
export function getDraftText() {
	return draftText;
}

/** Check if the effective model for this tab is currently loading (not ready for inference). */
export function isEffectiveModelLoading(): boolean {
	const modelId = getEffectiveModelId();
	if (!modelId) return false;
	// Check per-engine status
	if (engineStatus?.engines) {
		const engine = engineStatus.engines.find((e) => e.model_id === modelId);
		if (engine) {
			return engine.engine_state === 'starting';
		}
	}
	// If model is not in engines at all, check if it's loaded
	const model = models.find((m) => m.id === modelId);
	return model ? !model.loaded : false;
}

// --- Actions ---

export function toggleSidebar() {
	sidebarOpen = !sidebarOpen;
}

export function toggleSettings() {
	settingsOpen = !settingsOpen;
}

export function openSettings() {
	settingsOpen = true;
}

export function closeSettings() {
	settingsOpen = false;
}

export function openSettingsToTab(tab: SettingsTab) {
	settingsInitialTab = tab;
	settingsOpen = true;
}

export function clearChatError() {
	chatError = null;
	sendFailed = null;
}

export function clearDraftRestore() {
	draftText = '';
	draftImages = [];
	draftDocuments = [];
	sendFailed = null;
}

export function restoreDraftAttachments() {
	if (draftImages.length > 0 && pendingImages.length === 0) {
		pendingImages = draftImages.map((img) => ({
			file: img.file,
			preview: URL.createObjectURL(img.file)
		}));
	}
	if (draftDocuments.length > 0 && pendingDocuments.length === 0) {
		pendingDocuments = draftDocuments.map((doc) => ({
			file: doc.file,
			name: doc.name,
			ocrText: doc.ocrText
		}));
	}
}

/** Set the per-tab model override. This doesn't change the conversation's stored model. */
export function setTabModel(modelId: string | null) {
	tabModelId = modelId;
}

export function showNotification(message: string, type: 'error' | 'info' = 'error') {
	notification = { message, type };
	if (notificationTimeout) clearTimeout(notificationTimeout);
	notificationTimeout = setTimeout(() => {
		notification = null;
	}, 5000);
}

export function dismissNotification() {
	notification = null;
	if (notificationTimeout) {
		clearTimeout(notificationTimeout);
		notificationTimeout = null;
	}
}

export function applyTheme(theme: 'light' | 'dark' | 'system') {
	currentTheme = theme;
	let effective: 'light' | 'dark';
	if (theme === 'system') {
		effective = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
	} else {
		effective = theme;
	}
	document.documentElement.setAttribute('data-theme', effective);
}

/** Parse a hex color string into [r, g, b]. */
function hexToRgb(hex: string): [number, number, number] {
	hex = hex.replace('#', '');
	if (hex.length === 3) hex = hex[0]+hex[0]+hex[1]+hex[1]+hex[2]+hex[2];
	return [parseInt(hex.slice(0,2),16), parseInt(hex.slice(2,4),16), parseInt(hex.slice(4,6),16)];
}

/** Adjust brightness: positive = lighter, negative = darker. Amount in 0-1 range. */
function adjustBrightness(hex: string, amount: number): string {
	const [r,g,b] = hexToRgb(hex);
	const adjust = (c: number) => Math.max(0, Math.min(255, Math.round(c + (amount > 0 ? (255-c)*amount : c*amount))));
	return `#${adjust(r).toString(16).padStart(2,'0')}${adjust(g).toString(16).padStart(2,'0')}${adjust(b).toString(16).padStart(2,'0')}`;
}

/** Apply a custom accent color to CSS variables. Empty/undefined = use CSS defaults. */
export function applyAccentColor(hex?: string) {
	const root = document.documentElement.style;
	if (!hex) {
		root.removeProperty('--accent');
		root.removeProperty('--accent-hover');
		root.removeProperty('--accent-light');
		return;
	}
	const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
	const [r,g,b] = hexToRgb(hex);
	root.setProperty('--accent', hex);
	root.setProperty('--accent-hover', isDark ? adjustBrightness(hex, 0.2) : adjustBrightness(hex, -0.15));
	root.setProperty('--accent-light', isDark ? `rgba(${r},${g},${b},0.12)` : `rgba(${r},${g},${b},0.08)`);
}

/** Sync config state (pro_mode, pinned_models, theme, accent_color) and broadcast to other tabs. */
export function syncConfig(cfg: { pro_mode?: boolean; pinned_models?: string[]; theme?: string; accent_color?: string }) {
	if (cfg.pro_mode !== undefined) proMode = cfg.pro_mode;
	if (cfg.pinned_models !== undefined) pinnedModelIds = cfg.pinned_models;
	if (cfg.theme) applyTheme(cfg.theme as 'light' | 'dark' | 'system');
	if (cfg.accent_color !== undefined) applyAccentColor(cfg.accent_color || undefined);
	configChannel?.postMessage({ type: 'config-updated' });
}

export function cancelStream() {
	if (streamAbortController) {
		streamAbortController.abort();
		streamAbortController = null;
	}
}

export async function cancelQueueItem() {
	const id = queueItemId;
	if (id) {
		try {
			await api.cancelQueueItem(id);
		} catch (err) {
			console.error('Failed to cancel queue item:', err);
		}
	}
	// Also abort the SSE connection
	cancelStream();
	queuePosition = null;
	queueItemId = null;
}

export async function initApp() {
	initError = null;
	const maxRetries = 3;
	const retryDelay = 1000;

	// Listen for OS theme changes when using "system" theme
	window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
		if (currentTheme === 'system') {
			applyTheme('system');
			// Reapply accent color since hover shade depends on light/dark mode
			api.getConfig().then((cfg) => applyAccentColor(cfg.accent_color || undefined)).catch(() => {});
		}
	});

	// Listen for tab/window focus to check model availability
	document.addEventListener('visibilitychange', handleVisibilityChange);

	// Cross-tab config sync via BroadcastChannel
	if (!configChannel) {
		configChannel = new BroadcastChannel('fllint-config');
		configChannel.onmessage = (e) => {
			const msg = e.data;
			if (msg.type === 'config-updated') {
				// Refresh full config from server to get all changes
				api.getConfig().then((cfg) => {
					if (cfg.theme) applyTheme(cfg.theme as 'light' | 'dark' | 'system');
					applyAccentColor(cfg.accent_color || undefined);
					proMode = cfg.pro_mode ?? false;
					pinnedModelIds = cfg.pinned_models ?? [];
					responseBuffer = cfg.response_buffer ?? 2048;
				}).catch(() => {});
				// Refresh models in case loaded status changed
				api.listModels().then((m) => (models = m)).catch(() => {});
				api.fetchStatus().then((s) => (engineStatus = s)).catch(() => {});
			}
		};
	}

	// Fetch version (non-blocking, best-effort)
	api.getVersion().then((v) => { appVersion = v.version; }).catch(() => {});

	for (let attempt = 0; attempt < maxRetries; attempt++) {
		const results = await Promise.allSettled([
			api.listConversations(),
			api.listModels(),
			api.fetchStatus(),
			api.getConfig(),
			api.fetchMemory(),
			api.getDownloadRegistry(),
			api.getActiveDownloads(),
			api.listProviders()
		]);

		// Populate whatever succeeded, even if some calls failed/hung
		if (results[0].status === 'fulfilled') conversations = results[0].value;
		if (results[1].status === 'fulfilled') models = results[1].value;
		if (results[2].status === 'fulfilled') engineStatus = results[2].value;
		if (results[3].status === 'fulfilled') {
			const cfg = results[3].value;
			applyTheme(cfg.theme || 'system');
			applyAccentColor(cfg.accent_color || undefined);
			proMode = cfg.pro_mode ?? false;
			pinnedModelIds = cfg.pinned_models ?? [];
			responseBuffer = cfg.response_buffer ?? 2048;
		}
		if (results[4].status === 'fulfilled') memoryInfo = results[4].value;
		if (results[5].status === 'fulfilled') downloadRegistry = results[5].value;
		if (results[6].status === 'fulfilled') activeDownloads = results[6].value;
		if (results[7].status === 'fulfilled') providers = results[7].value;

		const allOk = results.every((r) => r.status === 'fulfilled');
		if (allOk) {
			initError = null;

			// Auto-start status polling if any engine is loading
			if (engineStatus?.engines?.some((e) => e.engine_state === 'starting')) {
				startStatusPolling();
			}
			// Auto-start download polling if any downloads are active
			if (activeDownloads.some((d) => d.state === 'downloading' || d.state === 'queued')) {
				startDownloadPolling();
			}
			// Check OCR availability (non-blocking)
			api.getHelperModels().then((data) => {
				const ocrSlot = data.slots.find((s: { slot: string }) => s.slot === 'OCR');
				ocrEnabled = !!(ocrSlot?.configured_model_id);
			}).catch(() => {});
			return;
		}

		// Some calls failed — retry unless this was the last attempt
		if (attempt < maxRetries - 1) {
			await new Promise((r) => setTimeout(r, retryDelay));
		}
	}
	// If we partially loaded data (models, conversations), don't show a full
	// init error — the user can still use whatever loaded.
	if (models.length === 0 && conversations.length === 0) {
		initError = 'Could not connect to Fllint server. Please restart the app or click Retry.';
	}
}

/**
 * When the tab regains focus, check if the currently selected model is still
 * loaded. If not, silently swap to the closest active model by size
 * that supports the same capabilities (e.g., vision).
 */
async function handleVisibilityChange() {
	if (document.visibilityState !== 'visible') return;

	// Refresh config (theme, pro_mode, pinned_models, accent_color may have changed in another tab)
	try {
		const cfg = await api.getConfig();
		if (cfg.theme && cfg.theme !== currentTheme) {
			applyTheme(cfg.theme as 'light' | 'dark' | 'system');
		}
		applyAccentColor(cfg.accent_color || undefined);
		proMode = cfg.pro_mode ?? false;
		pinnedModelIds = cfg.pinned_models ?? [];
		responseBuffer = cfg.response_buffer ?? 2048;
	} catch {
		// ignore
	}

	// Refresh models to get current loaded status
	try {
		models = await api.listModels();
	} catch {
		return;
	}

	const currentId = getEffectiveModelId();
	if (!currentId) return;

	const currentModel = models.find((m) => m.id === currentId);
	if (!currentModel || currentModel.loaded) return;

	// Current model is no longer loaded — find the closest active alternative
	const replacement = findClosestActiveModel(currentModel);
	if (replacement) {
		tabModelId = replacement.id;
	}
}

/**
 * Find the active (loaded) model closest in file size to the target,
 * preferring models that match capabilities (vision support).
 */
function findClosestActiveModel(target: ModelInfo): ModelInfo | null {
	const activeModels = models.filter((m) => m.loaded && m.id !== target.id);
	if (activeModels.length === 0) return null;

	// If target has vision, prefer vision-capable models
	let candidates = activeModels;
	if (target.vision) {
		const visionModels = activeModels.filter((m) => m.vision);
		if (visionModels.length > 0) candidates = visionModels;
	}

	const targetSize = target.size ?? 0;
	candidates.sort((a, b) => {
		const diffA = Math.abs((a.size ?? 0) - targetSize);
		const diffB = Math.abs((b.size ?? 0) - targetSize);
		return diffA - diffB;
	});

	return candidates[0];
}

export function addPendingImage(file: File) {
	pendingImages = [...pendingImages, { file, preview: URL.createObjectURL(file) }];
}

export function removePendingImage(index: number) {
	const removed = pendingImages[index];
	if (removed) {
		URL.revokeObjectURL(removed.preview);
	}
	pendingImages = pendingImages.filter((_, i) => i !== index);
}

export function clearPendingImages() {
	pendingImages.forEach((img) => URL.revokeObjectURL(img.preview));
	pendingImages = [];
}

export function getPendingDocuments() {
	return pendingDocuments;
}

export function addPendingDocument(file: File) {
	if (pendingDocuments.some((d) => d.name === file.name)) return;
	pendingDocuments = [...pendingDocuments, { file, name: file.name }];
}

export function removePendingDocument(index: number) {
	pendingDocuments = pendingDocuments.filter((_, i) => i !== index);
}

export function clearPendingDocuments() {
	pendingDocuments = [];
}

// --- OCR ---

export function isOcrEnabled() {
	return ocrEnabled;
}

export function setOcrEnabled(enabled: boolean) {
	ocrEnabled = enabled;
}

export function getOcrPopup() {
	return ocrPopup;
}

export function getOcrProgress() {
	return ocrProgress;
}

export function isOcrProcessing() {
	return pendingDocuments.some((d) => d.ocrProcessing);
}

export async function openOcrPopup(docIndex: number) {
	const doc = pendingDocuments[docIndex];
	if (!doc) return;

	// If already have popup data for this doc (re-opening during processing), just show it
	if (ocrPopup && ocrPopup.docIndex === docIndex) return;

	try {
		const { getPageCount } = await import('$lib/pdf');
		const pageCount = await getPageCount(doc.file);
		ocrPopup = { docIndex, filename: doc.name, file: doc.file, pageCount };
	} catch (err) {
		showNotification('Failed to read PDF pages.');
	}
}

export function closeOcrPopup() {
	// Don't clear progress/polling if OCR is still running — just hide the popup
	if (ocrProgress?.status === 'processing') {
		ocrPopup = null;
		return;
	}
	ocrPopup = null;
	ocrProgress = null;
	ocrJobId = null;
	if (ocrPollTimer) {
		clearInterval(ocrPollTimer);
		ocrPollTimer = null;
	}
}

export async function startOcrProcessing(pages: number[]) {
	if (!ocrPopup) return;
	const { docIndex, file, pageCount } = ocrPopup;

	// Mark document as processing
	pendingDocuments = pendingDocuments.map((d, i) =>
		i === docIndex ? { ...d, ocrProcessing: true } : d
	);

	try {
		// Upload the PDF first to get its URL (needed by backend for text extraction)
		const pdfUpload = await api.uploadDocument(file);
		const pdfUrl = pdfUpload.url;

		// Cache the upload result on the pending document
		pendingDocuments = pendingDocuments.map((d, i) =>
			i === docIndex ? { ...d, uploadedUrl: pdfUrl } : d
		);

		// Render selected OCR pages to images and upload them
		const { renderPageToBlob } = await import('$lib/pdf');
		const ocrPages: { page_num: number; image_url: string }[] = [];

		for (const pageNum of pages) {
			const blob = await renderPageToBlob(file, pageNum);
			const imageFile = new File([blob], `page-${pageNum}.png`, { type: 'image/png' });
			const result = await api.uploadImage(imageFile);
			ocrPages.push({ page_num: pageNum, image_url: result.url });
		}

		// Start OCR processing — backend handles ALL pages (OCR + text extraction)
		const { job_id } = await api.startOCR(pdfUrl, pageCount, ocrPages);
		ocrJobId = job_id;
		ocrProgress = {
			id: job_id,
			status: 'processing',
			total_pages: pageCount,
			done_pages: 0
		};

		// Poll for progress
		ocrPollTimer = setInterval(async () => {
			if (!ocrJobId) return;
			try {
				const status = await api.getOCRStatus(ocrJobId);
				ocrProgress = status;

				if (status.status === 'complete') {
					const warningMsg = status.failed_pages?.length
						? `OCR complete. Pages ${status.failed_pages.join(', ')} failed — text extraction was used as fallback.`
						: 'OCR processing complete.';
					pendingDocuments = pendingDocuments.map((d, i) =>
						i === docIndex
							? { ...d, ocrText: status.result_text, ocrProcessing: false }
							: d
					);
					// Clean up polling
					if (ocrPollTimer) {
						clearInterval(ocrPollTimer);
						ocrPollTimer = null;
					}
					ocrJobId = null;
					ocrProgress = null;
					ocrPopup = null;
					showNotification(warningMsg, status.failed_pages?.length ? 'error' : 'info');
				} else if (status.status === 'error') {
					pendingDocuments = pendingDocuments.map((d, i) =>
						i === docIndex ? { ...d, ocrProcessing: false } : d
					);
					if (ocrPollTimer) {
						clearInterval(ocrPollTimer);
						ocrPollTimer = null;
					}
					ocrJobId = null;
					showNotification(status.error || 'OCR processing failed.');
				} else if (status.status === 'cancelled') {
					pendingDocuments = pendingDocuments.map((d, i) =>
						i === docIndex ? { ...d, ocrProcessing: false } : d
					);
					if (ocrPollTimer) {
						clearInterval(ocrPollTimer);
						ocrPollTimer = null;
					}
					ocrJobId = null;
					ocrProgress = null;
					ocrPopup = null;
				}
			} catch {
				// Polling error, ignore
			}
		}, 1000);
	} catch (err) {
		pendingDocuments = pendingDocuments.map((d, i) =>
			i === docIndex ? { ...d, ocrProcessing: false } : d
		);
		showNotification(err instanceof Error ? err.message : 'OCR processing failed.');
	}
}

export async function cancelOcrProcessing() {
	if (ocrJobId) {
		try {
			await api.cancelOCR(ocrJobId);
		} catch {
			// Ignore
		}
	}
	if (ocrPopup) {
		const { docIndex } = ocrPopup;
		pendingDocuments = pendingDocuments.map((d, i) =>
			i === docIndex ? { ...d, ocrProcessing: false } : d
		);
	}
	ocrPopup = null;
	ocrProgress = null;
	ocrJobId = null;
	if (ocrPollTimer) {
		clearInterval(ocrPollTimer);
		ocrPollTimer = null;
	}
}

export async function loadConversations() {
	try {
		conversations = await api.listConversations();
	} catch (err) {
		console.error('Failed to load conversations:', err);
		showNotification('Failed to load conversations.');
	}
}

export async function selectConversation(id: string) {
	activeConversationId = id;
	tabModelId = null; // reset per-tab override when switching conversations
	contextUsage = null;
	lastResponseTruncated = false;
	try {
		const conv = await api.getConversation(id);
		messages = conv.messages;
		conversationModelId = conv.model_id ?? null;
	} catch (err) {
		console.error('Failed to load conversation:', err);
		showNotification('Failed to load conversation.');
	}
}

/** Navigate to a conversation's URL. Use this from sidebar/UI clicks. */
export function navigateToConversation(id: string) {
	goto(`/chat/${id}`);
}

export function newConversation() {
	activeConversationId = null;
	messages = [];
	chatError = null;
	tabModelId = null;
	conversationModelId = null;
	contextUsage = null;
	lastResponseTruncated = false;
}

/** Navigate to new chat and reset state. Use this from sidebar/UI clicks. */
export async function navigateToNewConversation() {
	// Navigate first to avoid race condition: if we're on /chat/[id],
	// calling newConversation() first clears activeConversationId which
	// triggers the chat page's $effect to re-load the conversation.
	await goto('/');
	newConversation();
}

export async function deleteConversation(id: string) {
	try {
		await api.deleteConversation(id);
		if (activeConversationId === id) {
			activeConversationId = null;
			messages = [];
			goto('/');
		}
		await loadConversations();
	} catch (err) {
		console.error('Failed to delete conversation:', err);
		showNotification('Failed to delete conversation.');
	}
}

export async function sendMessage(content: string, opts?: { noReasoning?: boolean; retry?: boolean }) {
	chatError = null;
	lastResponseTruncated = false;

	const noReasoning = opts?.noReasoning ?? false;
	const isRetry = opts?.retry ?? false;

	// Cache draft for restoration on failure (skip for retries and answerNow)
	if (!isRetry && !noReasoning) {
		draftText = content;
		draftImages = pendingImages.map((img) => ({ file: img.file }));
		draftDocuments = pendingDocuments.map((doc) => ({
			file: doc.file,
			name: doc.name,
			ocrText: doc.ocrText
		}));
	}
	sendFailed = null;

	// Check if context limit is reached before sending
	if (contextUsage && contextUsage.contextSize > 0) {
		const totalUsed = contextUsage.promptTokens + contextUsage.completionTokens;
		const limit = contextUsage.contextSize - responseBuffer;
		if (totalUsed > limit) {
			chatError = 'context_limit_reached';
			sendFailed = 'pre-stream';
			return;
		}
	}

	// Upload pending images and documents first (skip if re-sending)
	let imageUrls: string[] = [];
	let documentAttachments: DocumentAttachment[] = [];
	if (!noReasoning && !isRetry) {
		if (pendingImages.length > 0) {
			try {
				const uploads = await Promise.all(
					pendingImages.map((img) => api.uploadImage(img.file))
				);
				imageUrls = uploads.map((result) => result.url);
			} catch {
				chatError = 'Failed to upload image. Please try again.';
				sendFailed = 'pre-stream';
				return;
			}
			clearPendingImages();
		}

		if (pendingDocuments.length > 0) {
			try {
				const uploads = await Promise.all(
					pendingDocuments.map((doc) => api.uploadDocument(doc.file))
				);
				documentAttachments = uploads.map((result, i) => ({
					filename: result.original_name,
					url: result.url,
					text: pendingDocuments[i]?.ocrText ?? result.extracted_text
				}));
			} catch (err) {
				chatError = err instanceof Error ? err.message : 'Failed to upload document. Please try again.';
				sendFailed = 'pre-stream';
				return;
			}
			clearPendingDocuments();
		}

		// Build user message with images and documents for local display
		const userMsg: ChatMessage = { role: 'user', content };
		if (imageUrls.length > 0) {
			userMsg.images = imageUrls;
		}
		if (documentAttachments.length > 0) {
			userMsg.documents = documentAttachments;
		}
		messages = [...messages, userMsg];

		// Track what was sent so answerNow/retry can re-use it
		lastSentContent = content;
		lastSentImages = imageUrls;
		lastSentDocuments = documentAttachments;
	} else {
		// Re-sending: use the previously tracked images and documents
		imageUrls = lastSentImages;
		documentAttachments = lastSentDocuments;
	}

	isStreaming = true;
	streamingContent = '';
	streamingReasoning = '';
	thinkingDuration = null;
	streamAbortController = new AbortController();
	queuePosition = null;
	queueItemId = null;

	const effectiveModelId = getEffectiveModelId();
	const isNewConversation = !activeConversationId;

	try {
		const streamOpts = noReasoning
			? { noReasoning: true, retry: true }
			: isRetry
				? { retry: true }
				: undefined;

		for await (const token of api.streamChat(
			content,
			activeConversationId ?? undefined,
			imageUrls.length > 0 ? imageUrls : undefined,
			documentAttachments.length > 0 ? documentAttachments : undefined,
			effectiveModelId ?? undefined,
			streamAbortController.signal,
			streamOpts
		)) {
			if (token.conversation_id && !activeConversationId) {
				activeConversationId = token.conversation_id;
				// Navigate to the new conversation URL
				goto(`/chat/${token.conversation_id}`, { replaceState: true });
			}
			if (token.queue_id) {
				queueItemId = token.queue_id;
			}
			if (token.position !== undefined) {
				queuePosition = token.position;
			}
			if (token.reasoning) {
				streamingReasoning += token.reasoning;
			}
			if (token.thinking_duration !== undefined) {
				thinkingDuration = token.thinking_duration;
			}
			if (token.content) {
				// Once we receive content, we're no longer queued
				queuePosition = null;
				streamingContent += token.content;
			}
			if (token.usage) {
				contextUsage = {
					promptTokens: token.usage.prompt_tokens,
					completionTokens: token.usage.completion_tokens,
					contextSize: token.usage.context_size,
					finishReason: token.usage.finish_reason,
				};
			}
			if (token.error) {
				chatError = token.error;
			}
		}
		lastResponseTruncated = contextUsage?.finishReason === 'length';
		if (streamingContent) {
			const msg: ChatMessage = { role: 'assistant', content: streamingContent };
			if (streamingReasoning) {
				msg.reasoning = streamingReasoning;
			}
			if (thinkingDuration !== null) {
				msg.thinking_duration = thinkingDuration;
			}
			messages = [...messages, msg];
			// Message delivered successfully — clear the draft cache
			draftText = '';
			draftImages = [];
			draftDocuments = [];
		}
		await loadConversations();
		// Title generation is async — re-poll to pick up AI-generated title
		if (isNewConversation) {
			setTimeout(() => loadConversations(), 3000);
			setTimeout(() => loadConversations(), 8000);
		}
	} catch (err) {
		if (err instanceof DOMException && err.name === 'AbortError') {
			if (answerNowRequested) {
				// Don't keep partial response — answerNow will re-send
				answerNowRequested = false;
				return;
			}
			// User cancelled — keep partial response only if there's actual content
			if (streamingContent) {
				const msg: ChatMessage = { role: 'assistant', content: streamingContent };
				if (streamingReasoning) {
					msg.reasoning = streamingReasoning;
				}
				if (thinkingDuration !== null) {
					msg.thinking_duration = thinkingDuration;
				}
				messages = [...messages, msg];
			}
		} else {
			const errorMessage = err instanceof Error ? err.message : 'Failed to get response.';
			chatError = errorMessage;
			sendFailed = 'stream';
		}
	} finally {
		// If an SSE error was received during streaming, mark as stream failure
		if (chatError && sendFailed === null) {
			sendFailed = 'stream';
		}
		isStreaming = false;
		streamingContent = '';
		streamingReasoning = '';
		thinkingDuration = null;
		streamAbortController = null;
		queuePosition = null;
		queueItemId = null;
	}
}

export function answerNow() {
	if (!isStreaming || !streamAbortController) return;

	// Set flag so abort handler knows not to keep partial response
	answerNowRequested = true;
	const content = lastSentContent;

	// Cancel the current stream
	streamAbortController.abort();

	// Re-send the same message without reasoning
	// Small delay to let the abort handler finish
	setTimeout(() => {
		sendMessage(content, { noReasoning: true });
	}, 50);
}

export async function retryLastMessage() {
	if (sendFailed !== 'stream') return;
	sendFailed = null;
	chatError = null;
	await sendMessage(lastSentContent, { retry: true });
}

export async function loadModels() {
	try {
		models = await api.listModels();
	} catch (err) {
		console.error('Failed to load models:', err);
		showNotification('Failed to load models.');
	}
}

export async function loadMemory() {
	try {
		memoryInfo = await api.fetchMemory();
	} catch (err) {
		console.error('Failed to load memory info:', err);
	}
}

export async function unloadModel(modelId: string) {
	try {
		await api.unloadModel(modelId);
		await Promise.all([loadModels(), loadStatus(), loadMemory()]);
	} catch (err) {
		const errorMessage = err instanceof Error ? err.message : 'Failed to unload model.';
		showNotification(errorMessage);
	}
}

/** Load a model for the current tab without changing the global default. */
export async function selectModelForTab(modelId: string) {
	tabModelId = modelId;

	// External models are always ready — no loading needed
	if (modelId.startsWith('ext:')) {
		return;
	}

	try {
		await api.loadModel(modelId);
		await Promise.all([loadModels(), loadStatus(), loadMemory()]);
		// Start polling if this model is still loading
		if (engineStatus?.engines?.some((e) => e.model_id === modelId && e.engine_state === 'starting')) {
			startStatusPolling();
		}
	} catch (err) {
		if (err instanceof InsufficientMemoryError) {
			if (proMode) {
				unloadPopup = {
					targetModelId: modelId,
					memoryError: err.info
				};
			} else {
				chatError = err.message;
			}
			return;
		}
		const errorMessage = err instanceof Error ? err.message : 'Failed to load model.';
		chatError = errorMessage;
	}
}

/** Set a model as the global default and load it. Used only for initial setup. */
export async function switchModel(modelId: string) {
	try {
		await api.setActiveModel(modelId);
		await Promise.all([loadModels(), loadStatus(), loadMemory()]);
		if (engineStatus?.engines?.some((e) => e.model_id === modelId && e.engine_state === 'starting')) {
			startStatusPolling();
		}
	} catch (err) {
		if (err instanceof InsufficientMemoryError) {
			if (proMode) {
				unloadPopup = {
					targetModelId: modelId,
					memoryError: err.info
				};
			} else {
				chatError = err.message;
			}
			return;
		}
		const errorMessage = err instanceof Error ? err.message : 'Failed to switch model.';
		chatError = errorMessage;
	}
}

export function dismissUnloadPopup() {
	unloadPopup = null;
}

/** Unload selected models, then retry loading the target model. */
export async function confirmUnloadAndLoad(modelIdsToUnload: string[]) {
	const targetId = unloadPopup?.targetModelId;
	unloadPopup = null;
	if (!targetId) return;

	try {
		// Unload selected models
		for (const id of modelIdsToUnload) {
			await api.unloadModel(id);
		}
		// Retry loading the target model
		tabModelId = targetId;
		await api.loadModel(targetId);
		await Promise.all([loadModels(), loadStatus(), loadMemory()]);
		if (engineStatus?.engines?.some((e) => e.engine_state === 'starting')) {
			startStatusPolling();
		}
	} catch (err) {
		if (err instanceof InsufficientMemoryError) {
			// Still not enough — re-show popup
			unloadPopup = {
				targetModelId: targetId,
				memoryError: err.info
			};
			return;
		}
		const errorMessage = err instanceof Error ? err.message : 'Failed to load model.';
		chatError = errorMessage;
	}
}

export async function refreshModels() {
	try {
		models = await api.refreshModels();
		await Promise.all([loadStatus(), loadMemory()]);
	} catch (err) {
		console.error('Failed to refresh models:', err);
		showNotification('Failed to refresh models.');
	}
}

// --- Status ---

export async function loadStatus() {
	try {
		engineStatus = await api.fetchStatus();
	} catch (err) {
		console.error('Failed to load status:', err);
		showNotification('Failed to load engine status.');
	}
}

export function startStatusPolling() {
	stopStatusPolling();
	statusPollTimer = setInterval(async () => {
		await Promise.all([loadStatus(), loadModels()]);
		// Stop polling when no engine is in 'starting' state
		const anyStarting = engineStatus?.engines?.some((e) => e.engine_state === 'starting');
		if (!anyStarting) {
			stopStatusPolling();
		}
	}, 1000);
}

export function stopStatusPolling() {
	if (statusPollTimer) {
		clearInterval(statusPollTimer);
		statusPollTimer = null;
	}
}

// --- Downloads ---

export async function loadDownloadRegistry() {
	try {
		downloadRegistry = await api.getDownloadRegistry();
	} catch (err) {
		console.error('Failed to load download registry:', err);
	}
}

export async function loadActiveDownloads() {
	try {
		activeDownloads = await api.getActiveDownloads();
	} catch (err) {
		console.error('Failed to load active downloads:', err);
	}
}

export async function startModelDownload(registryId: string) {
	// Guard: only allow downloads triggered by user interaction
	if (typeof registryId !== 'string' || !registryId) {
		console.warn('startModelDownload called with invalid registryId:', registryId);
		return;
	}
	try {
		await api.startDownload(registryId);
		await loadActiveDownloads();
		startDownloadPolling();
	} catch (err) {
		const msg = err instanceof Error ? err.message : 'Failed to start download.';
		showNotification(msg);
	}
}

export async function cancelModelDownload(downloadId: string) {
	try {
		await api.cancelDownload(downloadId);
		await loadActiveDownloads();
	} catch (err) {
		const msg = err instanceof Error ? err.message : 'Failed to cancel download.';
		showNotification(msg);
	}
}

export function startDownloadPolling() {
	stopDownloadPolling();
	downloadPollTimer = setInterval(async () => {
		await Promise.all([loadActiveDownloads(), loadDownloadRegistry()]);
		const anyActive = activeDownloads.some(
			(d) => d.state === 'downloading' || d.state === 'queued'
		);
		if (!anyActive) {
			stopDownloadPolling();
			// Refresh models since a download may have completed.
			// The backend rescans the models directory and returns the updated list.
			await refreshModels();
			// Also reload the plain model list to ensure the selector is up to date
			await loadModels();
		}
	}, 1000);
}

export function stopDownloadPolling() {
	if (downloadPollTimer) {
		clearInterval(downloadPollTimer);
		downloadPollTimer = null;
	}
}

// --- Providers ---

export async function loadProviders() {
	try {
		providers = await api.listProviders();
	} catch (err) {
		console.error('Failed to load providers:', err);
	}
}

// --- Bulk Operations ---

export async function deleteAllConversations() {
	try {
		await api.deleteAllConversations();
		conversations = [];
		activeConversationId = null;
		messages = [];
		goto('/');
	} catch (err) {
		console.error('Failed to delete all conversations:', err);
		showNotification('Failed to delete conversations.');
	}
}
