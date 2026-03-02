import type { Conversation, ChatMessage, ModelInfo } from './types';
import * as api from './api';

// --- Conversations ---
let conversations = $state<Conversation[]>([]);
let activeConversationId = $state<string | null>(null);
let messages = $state<ChatMessage[]>([]);

// --- Streaming ---
let isStreaming = $state(false);
let streamingContent = $state('');

// --- Models ---
let models = $state<ModelInfo[]>([]);

// --- UI ---
let sidebarOpen = $state(true);
let settingsOpen = $state(false);
let pendingImage = $state<{ file: File; preview: string } | null>(null);

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
export function getPendingImage() {
	return pendingImage;
}

// --- Actions ---

export function toggleSidebar() {
	sidebarOpen = !sidebarOpen;
}

export function toggleSettings() {
	settingsOpen = !settingsOpen;
}

export function setPendingImage(file: File | null) {
	if (pendingImage?.preview) {
		URL.revokeObjectURL(pendingImage.preview);
	}
	if (file) {
		pendingImage = { file, preview: URL.createObjectURL(file) };
	} else {
		pendingImage = null;
	}
}

export async function loadConversations() {
	try {
		conversations = await api.listConversations();
	} catch (err) {
		console.error('Failed to load conversations:', err);
	}
}

export async function selectConversation(id: string) {
	activeConversationId = id;
	try {
		const conv = await api.getConversation(id);
		messages = conv.messages;
	} catch (err) {
		console.error('Failed to load conversation:', err);
	}
}

export function newConversation() {
	activeConversationId = null;
	messages = [];
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
	}
}

export async function sendMessage(content: string) {
	messages = [...messages, { role: 'user', content }];
	isStreaming = true;
	streamingContent = '';

	try {
		for await (const token of api.streamChat(content, activeConversationId ?? undefined)) {
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
		console.error('Stream error:', err);
		messages = [...messages, { role: 'assistant', content: 'Error: Failed to get response.' }];
	} finally {
		isStreaming = false;
		streamingContent = '';
	}
}

export async function loadModels() {
	try {
		models = await api.listModels();
	} catch (err) {
		console.error('Failed to load models:', err);
	}
}

export async function switchModel(modelId: string) {
	try {
		await api.setActiveModel(modelId);
		await loadModels();
	} catch (err) {
		console.error('Failed to switch model:', err);
	}
}
