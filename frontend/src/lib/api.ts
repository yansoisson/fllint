import type {
	Conversation,
	ModelInfo,
	AppConfig,
	SSEToken,
	ImageUploadResult,
	EngineStatus,
	MemoryInfo,
	MemoryErrorInfo,
	RegistryModel,
	DownloadStatus
} from './types';

const BASE = '/api';

export class InsufficientMemoryError extends Error {
	info: MemoryErrorInfo;
	constructor(info: MemoryErrorInfo) {
		super(info.model_name ? `Not enough memory to load ${info.model_name}` : 'Not enough memory');
		this.info = info;
	}
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		headers: { 'Content-Type': 'application/json' },
		...init
	});
	if (!res.ok) {
		const body = await res.json().catch(() => null);
		throw new Error(body?.error ?? `Request failed (${res.status})`);
	}
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
	conversationId?: string,
	images?: string[],
	modelId?: string,
	signal?: AbortSignal,
	opts?: { noReasoning?: boolean; retry?: boolean }
): AsyncGenerator<SSEToken, void, undefined> {
	const res = await fetch(`${BASE}/chat`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			content,
			conversation_id: conversationId ?? '',
			images: images?.length ? images : undefined,
			model_id: modelId || undefined,
			no_reasoning: opts?.noReasoning || undefined,
			retry: opts?.retry || undefined
		}),
		signal
	});

	if (!res.ok) {
		const body = await res.json().catch(() => null);
		throw new Error(body?.error ?? `Chat error: ${res.status}`);
	}
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
	const res = await fetch(`${BASE}/models/active`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ model_id: modelId })
	});
	if (!res.ok) {
		const body = await res.json().catch(() => null);
		if (body?.code === 'insufficient_memory') {
			throw new InsufficientMemoryError({
				model_name: body.model_name,
				required_bytes: body.required_bytes,
				available_bytes: body.available_bytes
			});
		}
		throw new Error(body?.error ?? `Request failed (${res.status})`);
	}
}

export async function refreshModels(): Promise<ModelInfo[]> {
	return request('/models/refresh', { method: 'POST' });
}

export async function loadModel(modelId: string): Promise<void> {
	const res = await fetch(`${BASE}/models/load`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ model_id: modelId })
	});
	if (!res.ok) {
		const body = await res.json().catch(() => null);
		if (body?.code === 'insufficient_memory') {
			throw new InsufficientMemoryError({
				model_name: body.model_name,
				required_bytes: body.required_bytes,
				available_bytes: body.available_bytes
			});
		}
		throw new Error(body?.error ?? `Request failed (${res.status})`);
	}
}

export async function unloadModel(modelId: string): Promise<void> {
	await request('/models/unload', {
		method: 'POST',
		body: JSON.stringify({ model_id: modelId })
	});
}

// --- Status ---

export async function fetchStatus(): Promise<EngineStatus> {
	return request('/status');
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

// --- System Prompt ---

export async function getDefaultSystemPrompt(): Promise<string> {
	const res = await request<{ prompt: string }>('/config/system-prompt-default');
	return res.prompt;
}

// --- Model Management ---

export async function deleteModel(modelId: string): Promise<void> {
	await request('/models/delete', {
		method: 'POST',
		body: JSON.stringify({ model_id: modelId })
	});
}

export async function renameModel(modelId: string, name: string): Promise<void> {
	await request('/models/rename', {
		method: 'POST',
		body: JSON.stringify({ model_id: modelId, name })
	});
}

// --- Folder ---

export async function openFolder(folder: 'models' | 'data'): Promise<void> {
	await request('/open-folder', {
		method: 'POST',
		body: JSON.stringify({ folder })
	});
}

// --- Memory ---

export async function fetchMemory(): Promise<MemoryInfo> {
	return request('/memory');
}

// --- Queue ---

export async function cancelQueueItem(id: string): Promise<void> {
	await fetch(`${BASE}/queue/${id}`, { method: 'DELETE' });
}

// --- Downloads ---

export async function getDownloadRegistry(): Promise<RegistryModel[]> {
	return request('/downloads/registry');
}

export async function startDownload(registryId: string): Promise<DownloadStatus> {
	return request('/downloads/start', {
		method: 'POST',
		body: JSON.stringify({ registry_id: registryId })
	});
}

export async function getActiveDownloads(): Promise<DownloadStatus[]> {
	return request('/downloads/active');
}

export async function cancelDownload(downloadId: string): Promise<void> {
	await request('/downloads/cancel', {
		method: 'POST',
		body: JSON.stringify({ download_id: downloadId })
	});
}

// --- Bulk Operations ---

export async function deleteAllConversations(): Promise<void> {
	await fetch(`${BASE}/conversations`, { method: 'DELETE' });
}
