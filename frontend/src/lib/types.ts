export interface ChatMessage {
	role: 'user' | 'assistant' | 'system';
	content: string;
}

export interface Conversation {
	id: string;
	title: string;
	messages: ChatMessage[];
	created_at: string;
	updated_at: string;
}

export interface ModelInfo {
	id: string;
	name: string;
	tier: 'lite' | 'standard' | 'pro';
	file_path?: string;
	size?: number;
	active: boolean;
}

export interface AppConfig {
	port: number;
	data_dir: string;
	models_dir: string;
}

export interface SSEToken {
	content?: string;
	conversation_id?: string;
}

export interface ImageUploadResult {
	id: string;
	filename: string;
	url: string;
}
