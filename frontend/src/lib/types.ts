export interface DocumentAttachment {
	filename: string;
	url: string;
	text: string;
}

export interface DocumentUploadResult {
	id: string;
	filename: string;
	url: string;
	original_name: string;
	extracted_text: string;
}

export interface ChatMessage {
	role: 'user' | 'assistant' | 'system';
	content: string;
	reasoning?: string;
	thinking_duration?: number;
	images?: string[];
	documents?: DocumentAttachment[];
}

export interface Conversation {
	id: string;
	title: string;
	model_id?: string;
	app_type?: string;
	messages: ChatMessage[];
	created_at: string;
	updated_at: string;
}

export interface ModelInfo {
	id: string;
	name: string;
	tier: 'lite' | 'standard' | 'pro' | 'custom' | 'external';
	file_path?: string;
	size?: number;
	active: boolean;
	loaded: boolean;
	vision: boolean;
	external: boolean;
	provider_id?: string;
}

export interface AppConfig {
	port: number;
	data_dir: string;
	models_dir: string;
	theme: 'light' | 'dark' | 'system';
	accent_color?: string;
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
	response_buffer: number;
	pinned_models?: string[];
	default_model_id?: string;
	forward_params_to_external: boolean;
	web_search_enabled?: boolean;
	ollama_api_key?: string;
	summary_model_id?: string;
	ocr_model_id?: string;
}

export interface OcrJobStatus {
	id: string;
	status: 'processing' | 'complete' | 'error' | 'cancelled';
	total_pages: number;
	done_pages: number;
	failed_pages?: number[];
	error?: string;
	result_text?: string;
	result_url?: string;
}

export interface SSEToken {
	content?: string;
	reasoning?: string;
	thinking_duration?: number;
	tool_status?: string;
	conversation_id?: string;
	queue_id?: string;
	position?: number;
	error?: string;
	code?: string;
	usage?: {
		prompt_tokens: number;
		completion_tokens: number;
		context_size: number;
		finish_reason: string;
	};
}

export interface EngineStatusInfo {
	model_id: string;
	model_name: string;
	engine_state: 'idle' | 'starting' | 'ready' | 'error' | 'stopping';
	error?: string;
	has_vision: boolean;
	load_progress?: number;
	context_size?: number;
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
	category: 'main' | 'helper';
	tier: 'lite' | 'standard' | 'pro' | 'helper';
	size: number;
	downloaded: boolean;
	mmproj_missing?: boolean;
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

export interface ProviderTypeInfo {
	type: string;
	label: string;
	requires_key: boolean;
	default_url: string;
}

export interface SelectedModel {
	name: string;
	display_name?: string;
	roles?: string[]; // e.g. ["main"], ["summary"], ["main","summary"]
}

export interface Provider {
	id: string;
	name: string;
	type: string;
	base_url: string;
	has_api_key: boolean;
	enabled: boolean;
	models: SelectedModel[];
}

export interface HelperModelOption {
	id: string;
	name: string;
	size?: number;
	external: boolean;
}

export interface HelperSlotInfo {
	slot: string;
	available_models: HelperModelOption[];
	configured_model_id: string;
	enabled: boolean;
}

export interface ProviderModel {
	name: string;
	size: number;
	details?: {
		family: string;
		parameter_size: string;
		quantization_level: string;
	};
}
