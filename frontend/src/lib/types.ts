export interface ChatMessage {
	role: 'user' | 'assistant' | 'system';
	content: string;
	reasoning?: string;
	thinking_duration?: number;
	images?: string[];
}

export interface Conversation {
	id: string;
	title: string;
	model_id?: string;
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
	loaded: boolean;
	vision: boolean;
}

export interface AppConfig {
	port: number;
	data_dir: string;
	models_dir: string;
	theme: 'light' | 'dark' | 'system';
	pro_mode: boolean;
	custom_instructions: string;
	temperature: number;
	top_p: number;
	top_k: number;
	repeat_penalty: number;
	max_tokens: number;
	seed: number;
	ctx_size: number;
	n_gpu_layers: number;
	flash_attn: 'auto' | 'on' | 'off';
	pinned_models?: string[];
}

export interface SSEToken {
	content?: string;
	reasoning?: string;
	thinking_duration?: number;
	conversation_id?: string;
	queue_id?: string;
	position?: number;
	error?: string;
	code?: string;
}

export interface EngineStatusInfo {
	model_id: string;
	model_name: string;
	engine_state: 'idle' | 'starting' | 'ready' | 'error' | 'stopping';
	error?: string;
	has_vision: boolean;
	load_progress?: number;
}

export interface EngineStatus {
	engines: EngineStatusInfo[];
	default_model_id?: string;
	has_binary: boolean;
	has_models: boolean;
	// Backward-compat fields from the default engine
	engine_state: 'idle' | 'starting' | 'ready' | 'error' | 'stopping';
	error?: string;
	model_name?: string;
	has_vision: boolean;
	load_progress?: number;
}

export interface ImageUploadResult {
	id: string;
	filename: string;
	url: string;
}

export interface MemoryErrorInfo {
	model_name: string;
	required_bytes: number;
	available_bytes: number;
}

export interface RegistryModel {
	id: string;
	display_name: string;
	tier: 'lite' | 'standard' | 'pro';
	size: number;
	downloaded: boolean;
	mmproj_size?: number;
}

export interface DownloadStatus {
	id: string;
	registry_id: string;
	display_name: string;
	total_bytes: number;
	done_bytes: number;
	state: 'queued' | 'downloading' | 'complete' | 'cancelled' | 'error';
	error?: string;
}

export interface MemoryInfo {
	system: {
		total_ram: number;
		available_ram: number;
		total_vram: number;
		available_vram: number;
		is_unified: boolean;
	} | null;
	used_by_models: number;
	models: {
		model_id: string;
		model_name: string;
		estimated_ram: number;
		loaded: boolean;
	}[];
}

export interface VersionInfo {
	version: string;
	build: string;
}
