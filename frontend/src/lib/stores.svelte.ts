import type { Conversation, ChatMessage, ModelInfo, EngineStatus } from './types';
import * as api from './api';

// --- Conversations ---
let conversations = $state<Conversation[]>([]);
let activeConversationId = $state<string | null>(null);
let messages = $state<ChatMessage[]>([]);

// --- Streaming ---
let isStreaming = $state(false);
let streamingContent = $state('');
let streamAbortController: AbortController | null = null;

// --- Models ---
let models = $state<ModelInfo[]>([]);

// --- Engine Status ---
let engineStatus = $state<EngineStatus | null>(null);
let statusPollTimer: ReturnType<typeof setInterval> | null = null;

// --- Theme ---
let currentTheme = $state<'light' | 'dark' | 'system'>('system');

// --- UI ---
let sidebarOpen = $state(true);
let settingsOpen = $state(false);
let pendingImages = $state<{ file: File; preview: string }[]>([]);
let chatError = $state<string | null>(null);
let initError = $state<string | null>(null);
let notification = $state<{ message: string; type: 'error' | 'info' } | null>(null);
let notificationTimeout: ReturnType<typeof setTimeout> | null = null;

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

// --- Actions ---

export function toggleSidebar() {
	sidebarOpen = !sidebarOpen;
}

export function toggleSettings() {
	settingsOpen = !settingsOpen;
}

export function clearChatError() {
	chatError = null;
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

export function cancelStream() {
	if (streamAbortController) {
		streamAbortController.abort();
		streamAbortController = null;
	}
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

	for (let attempt = 0; attempt < maxRetries; attempt++) {
		try {
			const [convs, mdls, status, cfg] = await Promise.all([
				api.listConversations(),
				api.listModels(),
				api.fetchStatus(),
				api.getConfig()
			]);
			conversations = convs;
			models = mdls;
			engineStatus = status;
			initError = null;

			// Apply theme from config
			applyTheme(cfg.theme || 'system');

			// Auto-start status polling if engine is already loading a model
			if (engineStatus?.engine_state === 'starting') {
				startStatusPolling();
			}
			return;
		} catch {
			if (attempt < maxRetries - 1) {
				await new Promise((r) => setTimeout(r, retryDelay));
			}
		}
	}
	initError = 'Could not connect to Fllint server. Please restart the app or click Retry.';
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
	try {
		const conv = await api.getConversation(id);
		messages = conv.messages;
	} catch (err) {
		console.error('Failed to load conversation:', err);
		showNotification('Failed to load conversation.');
	}
}

export function newConversation() {
	activeConversationId = null;
	messages = [];
	chatError = null;
}

export async function deleteConversation(id: string) {
	try {
		await api.deleteConversation(id);
		if (activeConversationId === id) {
			activeConversationId = null;
			messages = [];
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

	try {
		for await (const token of api.streamChat(
			content,
			activeConversationId ?? undefined,
			imageUrls.length > 0 ? imageUrls : undefined,
			streamAbortController.signal
		)) {
			if (token.conversation_id && !activeConversationId) {
				activeConversationId = token.conversation_id;
			}
			if (token.content) {
				streamingContent += token.content;
			}
		}
		messages = [...messages, { role: 'assistant', content: streamingContent }];
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

export async function switchModel(modelId: string) {
	try {
		await api.setActiveModel(modelId);
		await loadModels();
		await loadStatus();
		if (engineStatus?.engine_state === 'starting') {
			startStatusPolling();
		}
	} catch (err) {
		const errorMessage = err instanceof Error ? err.message : 'Failed to switch model.';
		chatError = errorMessage;
	}
}

export async function refreshModels() {
	try {
		models = await api.refreshModels();
		await loadStatus();
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
		await loadStatus();
		if (engineStatus?.engine_state !== 'starting') {
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
	} catch (err) {
		console.error('Failed to delete all conversations:', err);
		showNotification('Failed to delete conversations.');
	}
}
