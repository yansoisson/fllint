import type { Conversation, ChatMessage, ModelInfo, EngineStatus, MemoryInfo, MemoryErrorInfo } from './types';
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
let streamAbortController: AbortController | null = null;

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

// --- UI ---
let sidebarOpen = $state(true);
let settingsOpen = $state(false);
let pendingImages = $state<{ file: File; preview: string }[]>([]);
let chatError = $state<string | null>(null);
let initError = $state<string | null>(null);
let notification = $state<{ message: string; type: 'error' | 'info' } | null>(null);
let notificationTimeout: ReturnType<typeof setTimeout> | null = null;

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
export function getPinnedModelIds() {
	return pinnedModelIds;
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

export function clearChatError() {
	chatError = null;
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

/** Sync config state (pro_mode, pinned_models, theme) and broadcast to other tabs. */
export function syncConfig(cfg: { pro_mode?: boolean; pinned_models?: string[]; theme?: string }) {
	if (cfg.pro_mode !== undefined) proMode = cfg.pro_mode;
	if (cfg.pinned_models !== undefined) pinnedModelIds = cfg.pinned_models;
	if (cfg.theme) applyTheme(cfg.theme as 'light' | 'dark' | 'system');
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
					proMode = cfg.pro_mode ?? false;
					pinnedModelIds = cfg.pinned_models ?? [];
				}).catch(() => {});
				// Refresh models in case loaded status changed
				api.listModels().then((m) => (models = m)).catch(() => {});
				api.fetchStatus().then((s) => (engineStatus = s)).catch(() => {});
			}
		};
	}

	for (let attempt = 0; attempt < maxRetries; attempt++) {
		const results = await Promise.allSettled([
			api.listConversations(),
			api.listModels(),
			api.fetchStatus(),
			api.getConfig(),
			api.fetchMemory()
		]);

		// Populate whatever succeeded, even if some calls failed/hung
		if (results[0].status === 'fulfilled') conversations = results[0].value;
		if (results[1].status === 'fulfilled') models = results[1].value;
		if (results[2].status === 'fulfilled') engineStatus = results[2].value;
		if (results[3].status === 'fulfilled') {
			const cfg = results[3].value;
			applyTheme(cfg.theme || 'system');
			proMode = cfg.pro_mode ?? false;
			pinnedModelIds = cfg.pinned_models ?? [];
		}
		if (results[4].status === 'fulfilled') memoryInfo = results[4].value;

		const allOk = results.every((r) => r.status === 'fulfilled');
		if (allOk) {
			initError = null;

			// Auto-start status polling if any engine is loading
			if (engineStatus?.engines?.some((e) => e.engine_state === 'starting')) {
				startStatusPolling();
			}
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

	// Refresh config (theme, pro_mode, pinned_models may have changed in another tab)
	try {
		const cfg = await api.getConfig();
		if (cfg.theme && cfg.theme !== currentTheme) {
			applyTheme(cfg.theme as 'light' | 'dark' | 'system');
		}
		proMode = cfg.pro_mode ?? false;
		pinnedModelIds = cfg.pinned_models ?? [];
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
}

/** Navigate to new chat and reset state. Use this from sidebar/UI clicks. */
export function navigateToNewConversation() {
	newConversation();
	goto('/');
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

export async function sendMessage(content: string) {
	chatError = null;

	// Upload pending images first
	let imageUrls: string[] = [];
	if (pendingImages.length > 0) {
		try {
			const uploads = await Promise.all(
				pendingImages.map((img) => api.uploadImage(img.file))
			);
			imageUrls = uploads.map((result) => result.url);
		} catch {
			chatError = 'Failed to upload image. Please try again.';
			return;
		}
		clearPendingImages();
	}

	// Build user message with images for local display
	const userMsg: ChatMessage = { role: 'user', content };
	if (imageUrls.length > 0) {
		userMsg.images = imageUrls;
	}
	messages = [...messages, userMsg];

	isStreaming = true;
	streamingContent = '';
	streamAbortController = new AbortController();
	queuePosition = null;
	queueItemId = null;

	const effectiveModelId = getEffectiveModelId();

	try {
		for await (const token of api.streamChat(
			content,
			activeConversationId ?? undefined,
			imageUrls.length > 0 ? imageUrls : undefined,
			effectiveModelId ?? undefined,
			streamAbortController.signal
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
			if (token.content) {
				// Once we receive content, we're no longer queued
				queuePosition = null;
				streamingContent += token.content;
			}
			if (token.error) {
				chatError = token.error;
			}
		}
		if (streamingContent) {
			messages = [...messages, { role: 'assistant', content: streamingContent }];
		}
		await loadConversations();
	} catch (err) {
		if (err instanceof DOMException && err.name === 'AbortError') {
			// User cancelled — keep partial response if any
			if (streamingContent) {
				messages = [...messages, { role: 'assistant', content: streamingContent }];
			}
		} else {
			const errorMessage = err instanceof Error ? err.message : 'Failed to get response.';
			chatError = errorMessage;
		}
	} finally {
		isStreaming = false;
		streamingContent = '';
		streamAbortController = null;
		queuePosition = null;
		queueItemId = null;
	}
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
