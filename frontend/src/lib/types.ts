export interface ChatMessage {
	role: 'user' | 'assistant' | 'system';
	content: string;
	images?: string[];
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
	tier: 'lite' | 'standard' | 'pro' | 'custom';
	file_path?: string;
	size?: number;
	active: boolean;
	vision: boolean;
}

export interface AppConfig {
	port: number;
	data_dir: string;
	models_dir: string;
	theme: 'light' | 'dark' | 'system';
	pro_mode: boolean;
	custom_instructions: string;
	system_prompt: string;
	temperature: number;
	top_p: number;
	top_k: number;
	repeat_penalty: number;
	max_tokens: number;
	seed: number;
	ctx_size: number;
	n_gpu_layers: number;
	flash_attn: 'auto' | 'on' | 'off';
}

export interface SSEToken {
	content?: string;
	conversation_id?: string;
	error?: string;
	code?: string;
}

export interface EngineStatus {
	engine_state: 'idle' | 'starting' | 'ready' | 'error' | 'stopping';
	error?: string;
	model_name?: string;
	has_binary: boolean;
	has_models: boolean;
	has_vision: boolean;
}

export interface ImageUploadResult {
	id: string;
	filename: string;
	url: string;
}
