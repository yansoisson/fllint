import type {
	Conversation,
	ModelInfo,
	AppConfig,
	SSEToken,
	ImageUploadResult
} from './types';

const BASE = '/api';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		headers: { 'Content-Type': 'application/json' },
		...init
	});
	if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
	return res.json();
}

// --- Conversations ---

export async function listConversations(): Promise<Conversation[]> {
	return request('/conversations');
}

export async function getConversation(id: string): Promise<Conversation> {
	return request(`/conversations/${id}`);
}

export async function createConversation(title: string): Promise<Conversation> {
	return request('/conversations', {
		method: 'POST',
		body: JSON.stringify({ title })
	});
}

export async function deleteConversation(id: string): Promise<void> {
	await fetch(`${BASE}/conversations/${id}`, { method: 'DELETE' });
}

// --- Chat Streaming ---

export async function* streamChat(
	content: string,
	conversationId?: string
): AsyncGenerator<SSEToken, void, undefined> {
	const res = await fetch(`${BASE}/chat`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			content,
			conversation_id: conversationId ?? ''
		})
	});

	if (!res.ok) throw new Error(`Chat error: ${res.status}`);
	if (!res.body) throw new Error('No response body');

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	let buffer = '';

	while (true) {
		const { done, value } = await reader.read();
		if (done) break;

		buffer += decoder.decode(value, { stream: true });
		const lines = buffer.split('\n');
		buffer = lines.pop() ?? '';

		for (const line of lines) {
			if (!line.startsWith('data: ')) continue;
			const data = line.slice(6).trim();
			if (data === '[DONE]') return;
			try {
				yield JSON.parse(data) as SSEToken;
			} catch {
				// Skip malformed events
			}
		}
	}
}

// --- Models ---

export async function listModels(): Promise<ModelInfo[]> {
	return request('/models');
}

export async function setActiveModel(modelId: string): Promise<void> {
	await request('/models/active', {
		method: 'PUT',
		body: JSON.stringify({ model_id: modelId })
	});
}

// --- Image Upload ---

export async function uploadImage(file: File): Promise<ImageUploadResult> {
	const form = new FormData();
	form.append('image', file);
	const res = await fetch(`${BASE}/image/upload`, {
		method: 'POST',
		body: form
	});
	if (!res.ok) throw new Error('Upload failed');
	return res.json();
}

// --- Config ---

export async function getConfig(): Promise<AppConfig> {
	return request('/config');
}

export async function updateConfig(config: Partial<AppConfig>): Promise<AppConfig> {
	return request('/config', {
		method: 'PUT',
		body: JSON.stringify(config)
	});
}
