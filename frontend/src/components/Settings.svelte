<script lang="ts">
	import {
		getSettingsOpen,
		toggleSettings,
		applyTheme,
		applyAccentColor,
		syncConfig,
		getModels,
		loadModels,
		loadStatus,
		unloadModel,
		deleteAllConversations,
		showNotification,
		getConversations,
		getEngineStatus,
		getDownloadRegistry,
		getActiveDownloadsState,
		startModelDownload,
		cancelModelDownload,
		loadDownloadRegistry,
		getSettingsInitialTab,
		loadProviders,
		getProviders,
		setOcrEnabled
	} from '$lib/stores.svelte';
	import * as api from '$lib/api';
	import type { AppConfig, ModelInfo, DownloadStatus, Provider, ProviderTypeInfo, ProviderModel, SelectedModel, HelperSlotInfo } from '$lib/types';

	type SettingsTab = 'general' | 'models' | 'helper' | 'advanced';

	let config = $state<AppConfig | null>(null);
	let loading = $state(false);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let defaultPrompt = $state('');
	let systemPrompt = $state('');
	let systemPromptIsDefault = $state(true);
	let editingModelId = $state<string | null>(null);
	let editingName = $state('');
	let deleteModelConfirm = $state<string | null>(null);
	let deleteAllStep = $state(0);

	// --- Add model state ---
	let addModelOpen = $state(false);
	let addModelGguf = $state<File | null>(null);
	let addModelMmproj = $state<File | null>(null);
	let addModelName = $state('');
	let addModelUploading = $state(false);
	let deleteAllTimeout: ReturnType<typeof setTimeout> | null = null;
	let unloadingModelId = $state<string | null>(null);
	let dragModelId = $state<string | null>(null);
	let dragOverModelId = $state<string | null>(null);
	let activeTab = $state<SettingsTab>('general');
	let appVersion = $state<string | null>(null);
	let checkingUpdate = $state(false);

	// --- Provider state ---
	let providerTypes = $state<ProviderTypeInfo[]>([]);
	let addingProvider = $state(false);
	let editingProviderId = $state<string | null>(null);
	let providerForm = $state({
		name: '',
		type: 'ollama-local',
		base_url: 'http://localhost:11434',
		api_key: '',
		enabled: true
	});
	let testingConnection = $state(false);
	let testResult = $state<'success' | 'error' | null>(null);
	let testError = $state<string | null>(null);
	let fetchingModels = $state<string | null>(null); // provider ID being fetched
	let availableModels = $state<ProviderModel[]>([]);
	let selectedModelRoles = $state<Map<string, Set<string>>>(new Map());
	let savingModels = $state(false);

	const MODEL_ROLES = [
		{ id: 'main', label: 'Main' },
		{ id: 'summary', label: 'Summary' },
	] as const;
	let deletingProviderId = $state<string | null>(null);
	let managingModelsId = $state<string | null>(null);

	let allProviders = $derived(getProviders());

	// --- Helper models state ---
	let helperSlots = $state<HelperSlotInfo[]>([]);
	let helperLoading = $state(false);
	let savingHelper = $state(false);

	// Refresh helper model list when a helper model download completes
	let prevSummaryDownloaded = $state(false);
	$effect(() => {
		const reg = registryModels.find(m => m.id === 'helper-summary-qwen3.5-0.8b');
		const downloaded = reg?.downloaded ?? false;
		if (downloaded && !prevSummaryDownloaded && activeTab === 'helper') {
			loadHelperModels();
		}
		prevSummaryDownloaded = downloaded;
	});

	let prevOcrDownloaded = $state(false);
	$effect(() => {
		const reg = registryModels.find(m => m.id === 'helper-ocr-glm-ocr');
		const downloaded = reg?.downloaded ?? false;
		if (downloaded && !prevOcrDownloaded && activeTab === 'helper') {
			loadHelperModels();
		}
		prevOcrDownloaded = downloaded;
	});

	function getProviderTypeInfo(type: string): ProviderTypeInfo | undefined {
		return providerTypes.find((t) => t.type === type);
	}

	function resetProviderForm() {
		providerForm = { name: '', type: 'ollama-local', base_url: 'http://localhost:11434', api_key: '', enabled: true };
		testResult = null;
		testError = null;
		addingProvider = false;
		editingProviderId = null;
	}

	function startAddProvider() {
		resetProviderForm();
		addingProvider = true;
	}

	function startEditProvider(p: Provider) {
		editingProviderId = p.id;
		providerForm = {
			name: p.name,
			type: p.type,
			base_url: p.base_url,
			api_key: '',
			enabled: p.enabled
		};
		testResult = null;
		testError = null;
		addingProvider = false;
	}

	function handleProviderTypeChange() {
		const info = getProviderTypeInfo(providerForm.type);
		if (info?.default_url) {
			providerForm.base_url = info.default_url;
		}
	}

	async function saveProvider() {
		try {
			if (editingProviderId) {
				await api.updateProvider(editingProviderId, {
					name: providerForm.name,
					type: providerForm.type,
					base_url: providerForm.base_url,
					api_key: providerForm.api_key || undefined,
					enabled: providerForm.enabled
				});
			} else {
				await api.createProvider({
					name: providerForm.name,
					type: providerForm.type,
					base_url: providerForm.base_url,
					api_key: providerForm.api_key || undefined,
					enabled: providerForm.enabled
				});
			}
			resetProviderForm();
			await Promise.all([loadProviders(), loadModels()]);
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to save provider.';
			showNotification(msg, 'error');
		}
	}

	async function deleteProvider(id: string) {
		if (deletingProviderId !== id) {
			deletingProviderId = id;
			return;
		}
		try {
			await api.deleteProvider(id);
			deletingProviderId = null;
			if (managingModelsId === id) managingModelsId = null;
			await Promise.all([loadProviders(), loadModels()]);
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to delete provider.';
			showNotification(msg, 'error');
			deletingProviderId = null;
		}
	}

	async function testProviderConnection(id: string) {
		testingConnection = true;
		testResult = null;
		testError = null;
		try {
			await api.testProvider(id);
			testResult = 'success';
		} catch (err) {
			testResult = 'error';
			testError = err instanceof Error ? err.message : 'Connection failed.';
		} finally {
			testingConnection = false;
		}
	}

	async function toggleManageModels(p: Provider) {
		if (managingModelsId === p.id) {
			managingModelsId = null;
			return;
		}
		managingModelsId = p.id;
		fetchingModels = p.id;
		// Build role map from saved models
		const roleMap = new Map<string, Set<string>>();
		for (const m of p.models) {
			const roles = m.roles && m.roles.length > 0 ? m.roles : ['main'];
			roleMap.set(m.name, new Set(roles));
		}
		selectedModelRoles = roleMap;
		try {
			availableModels = (await api.fetchProviderModels(p.id)).sort((a, b) => a.name.localeCompare(b.name));
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to fetch models.';
			showNotification(msg, 'error');
			availableModels = [];
		} finally {
			fetchingModels = null;
		}
	}

	function toggleModelRole(name: string, role: string) {
		const next = new Map(selectedModelRoles);
		let roles = next.get(name);
		if (!roles) {
			roles = new Set<string>();
			next.set(name, roles);
		}
		const nextRoles = new Set(roles);
		if (nextRoles.has(role)) {
			nextRoles.delete(role);
		} else {
			nextRoles.add(role);
		}
		// If no roles remain, remove the model entirely
		if (nextRoles.size === 0) {
			next.delete(name);
		} else {
			next.set(name, nextRoles);
		}
		selectedModelRoles = next;
	}

	function isModelRoleSelected(name: string, role: string): boolean {
		return selectedModelRoles.get(name)?.has(role) ?? false;
	}

	async function saveProviderModels(providerId: string) {
		savingModels = true;
		try {
			const models: SelectedModel[] = [];
			for (const [name, roles] of selectedModelRoles) {
				if (roles.size > 0) {
					models.push({ name, roles: [...roles] });
				}
			}
			await api.saveProviderModels(providerId, models);
			await Promise.all([loadProviders(), loadModels()]);
			managingModelsId = null;
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to save models.';
			showNotification(msg, 'error');
		} finally {
			savingModels = false;
		}
	}

	async function toggleProviderEnabled(p: Provider) {
		try {
			await api.updateProvider(p.id, {
				name: p.name,
				type: p.type,
				base_url: p.base_url,
				enabled: !p.enabled
			});
			await Promise.all([loadProviders(), loadModels()]);
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to update provider.';
			showNotification(msg, 'error');
		}
	}

	// Loaded models derived from model list
	let loadedModels = $derived(getModels().filter((m) => m.loaded));
	let status = $derived(getEngineStatus());

	// Download state
	let registryModels = $derived(getDownloadRegistry());
	let downloads = $derived(getActiveDownloadsState());

	function isModelStarting(modelId: string): boolean {
		if (!status?.engines) return false;
		return status.engines.some((e) => e.model_id === modelId && e.engine_state === 'starting');
	}

	// Pinned model IDs (from config, or default tier models)
	let pinnedIds = $derived.by(() => {
		if (config?.pinned_models && config.pinned_models.length > 0) {
			return config.pinned_models;
		}
		return getModels()
			.filter((m) => m.tier === 'lite' || m.tier === 'standard' || m.tier === 'pro')
			.map((m) => m.id);
	});

	// Ordered pinned models
	let pinnedModels = $derived.by(() => {
		const allModels = getModels();
		const result: ModelInfo[] = [];
		for (const id of pinnedIds) {
			const m = allModels.find((model) => model.id === id);
			if (m) result.push(m);
		}
		return result;
	});

	// Unpinned models
	let unpinnedModels = $derived(getModels().filter((m) => !pinnedIds.includes(m.id)));

	function togglePin(model: ModelInfo) {
		if (!config) return;
		const currentPinned = [...(config.pinned_models && config.pinned_models.length > 0
			? config.pinned_models
			: pinnedIds)];

		if (currentPinned.includes(model.id)) {
			config.pinned_models = currentPinned.filter((id) => id !== model.id);
		} else {
			config.pinned_models = [...currentPinned, model.id];
		}
		api.updateConfig(config).then((c) => {
			config = c;
			syncConfig(c);
		});
	}

	async function setDefaultModel(modelId: string) {
		if (!config) return;
		config.default_model_id = modelId;

		// Auto-pin the new default model so it appears in the selector
		const currentPinned = config.pinned_models && config.pinned_models.length > 0
			? [...config.pinned_models]
			: [...pinnedIds];
		if (!currentPinned.includes(modelId)) {
			currentPinned.push(modelId);
		}
		config.pinned_models = currentPinned;

		try {
			config = await api.updateConfig(config);
			syncConfig(config);
			// Also set it as the active model in the manager
			await api.setActiveModel(modelId);
			await Promise.all([loadModels(), loadStatus()]);
			showNotification('Default model updated.', 'info');
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to set default model.';
			showNotification(msg, 'error');
		}
	}

	async function handleUnload(modelId: string) {
		unloadingModelId = modelId;
		try {
			await unloadModel(modelId);
		} finally {
			unloadingModelId = null;
		}
	}

	// Drag-and-drop reorder for pinned models
	function handleDragStart(modelId: string) {
		dragModelId = modelId;
	}

	function handleDragOver(e: DragEvent, modelId: string) {
		e.preventDefault();
		dragOverModelId = modelId;
	}

	function handleDragLeave() {
		dragOverModelId = null;
	}

	function handleDrop(e: DragEvent, targetId: string) {
		e.preventDefault();
		dragOverModelId = null;
		if (!dragModelId || dragModelId === targetId || !config) return;

		const currentPinned = [...(config.pinned_models && config.pinned_models.length > 0
			? config.pinned_models
			: pinnedIds)];

		const fromIdx = currentPinned.indexOf(dragModelId);
		const toIdx = currentPinned.indexOf(targetId);
		if (fromIdx === -1 || toIdx === -1) return;

		currentPinned.splice(fromIdx, 1);
		currentPinned.splice(toIdx, 0, dragModelId);
		config.pinned_models = currentPinned;
		api.updateConfig(config).then((c) => {
			config = c;
			syncConfig(c);
		});
		dragModelId = null;
	}

	function handleDragEnd() {
		dragModelId = null;
		dragOverModelId = null;
	}

	$effect(() => {
		if (getSettingsOpen()) {
			activeTab = getSettingsInitialTab();
			loadConfig();
		} else {
			config = null;
			error = null;
			editingModelId = null;
			deleteModelConfirm = null;
			deleteAllStep = 0;
			activeTab = 'general';
		}
	});

	// Redirect from Advanced tab when Pro Mode is toggled off
	$effect(() => {
		if (config && !config.pro_mode && activeTab === 'advanced') {
			activeTab = 'general';
		}
	});

	async function loadConfig() {
		loading = true;
		error = null;
		try {
			config = await api.getConfig();
		} catch (err) {
			console.error('Failed to load settings:', err);
			error = 'Failed to load settings. Please try again.';
		} finally {
			loading = false;
		}
		if (!config) return;
		customAccentInput = config.accent_color || '';
		// Load secondary data in the background — tab content is already visible
		Promise.all([loadModels(), loadStatus(), loadDownloadRegistry(), loadProviders()]).catch(() => {});
		api.listProviderTypes().then((t) => { providerTypes = t; }).catch(() => { providerTypes = []; });
		api.getSystemPrompt().then((sp) => {
			systemPrompt = sp.prompt;
			systemPromptIsDefault = sp.is_default;
		}).catch(() => {});
		api.getDefaultSystemPrompt().then((dp) => { defaultPrompt = dp; }).catch(() => { defaultPrompt = ''; });
		api.getVersion().then((v) => { appVersion = v.version; }).catch(() => { appVersion = null; });
	}

	async function loadHelperModels() {
		helperLoading = true;
		try {
			const data = await api.getHelperModels();
			helperSlots = data.slots;
		} catch (err) {
			console.error('Failed to load helper models:', err);
		} finally {
			helperLoading = false;
		}
	}

	async function saveHelperConfig(cfg: { summary_model_id?: string; ocr_model_id?: string }) {
		savingHelper = true;
		try {
			await api.updateHelperConfig(cfg);
			showNotification('Helper model configuration saved.', 'info');
		} catch (err) {
			console.error('Failed to save helper config:', err);
			showNotification('Failed to save helper model configuration.');
		} finally {
			savingHelper = false;
		}
	}

	async function save() {
		if (!config || saving) return;
		saving = true;
		error = null;
		try {
			const spResult = await api.updateSystemPrompt(systemPrompt);
			systemPromptIsDefault = spResult.is_default;
			config = await api.updateConfig(config);
			showNotification('Settings saved.', 'info');
		} catch (err) {
			console.error('Failed to save settings:', err);
			error = 'Failed to save settings. Please try again.';
		} finally {
			saving = false;
		}
	}

	async function handleCheckUpdate() {
		checkingUpdate = true;
		try {
			await api.checkForUpdate();
			showNotification('Checking for updates...', 'info');
		} catch (err: any) {
			showNotification(err.message || 'Failed to check for updates.', 'error');
		} finally {
			checkingUpdate = false;
		}
	}

	function setTheme(theme: 'light' | 'dark' | 'system') {
		if (!config) return;
		config.theme = theme;
		applyTheme(theme);
		// Reapply accent color since hover shade depends on light/dark mode
		applyAccentColor(config.accent_color || undefined);
		api.updateConfig(config).then((c) => {
			config = c;
			syncConfig(c);
		});
	}

	const accentPresets = [
		{ hex: '', label: 'Gray' },
		{ hex: '#3b82f6', label: 'Blue' },
		{ hex: '#8b5cf6', label: 'Purple' },
		{ hex: '#ef4444', label: 'Red' },
		{ hex: '#f97316', label: 'Orange' },
		{ hex: '#10a37f', label: 'Teal' },
		{ hex: '#ec4899', label: 'Pink' },
		{ hex: '#06b6d4', label: 'Cyan' },
	] as const;

	let customAccentInput = $state('');

	function setAccentColor(hex: string) {
		if (!config) return;
		config.accent_color = hex;
		applyAccentColor(hex || undefined);
		customAccentInput = hex;
		api.updateConfig(config).then((c) => {
			config = c;
			syncConfig(c);
		});
	}

	function handleCustomAccentInput(value: string) {
		customAccentInput = value;
		if (/^#[0-9a-fA-F]{6}$/.test(value)) {
			setAccentColor(value);
		}
	}

	function toggleProMode() {
		if (!config) return;
		config.pro_mode = !config.pro_mode;
		api.updateConfig(config).then((c) => {
			config = c;
			syncConfig(c);
		});
	}

	function formatSize(bytes?: number): string {
		if (!bytes) return '';
		const gb = bytes / (1024 * 1024 * 1024);
		if (gb >= 1) return gb.toFixed(1) + ' GB';
		const mb = bytes / (1024 * 1024);
		return mb.toFixed(0) + ' MB';
	}


	function findActiveDownload(registryId: string): DownloadStatus | undefined {
		return downloads.find(
			(d) => d.registry_id === registryId && d.state !== 'complete' && d.state !== 'cancelled'
		);
	}

	function progressPercent(dl: DownloadStatus): number {
		if (!dl.total_bytes) return 0;
		return Math.round((dl.done_bytes / dl.total_bytes) * 100);
	}

	async function handleStartDownload(registryId: string) {
		await startModelDownload(registryId);
	}

	async function handleCancelDownload(downloadId: string) {
		await cancelModelDownload(downloadId);
	}

	function autoDetectName(filename: string): string {
		let name = filename.replace(/\.gguf$/i, '');
		name = name.replace(/[-_.]/g, ' ');
		return name.split(/\s+/).map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
	}

	function handleGgufSelect(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0] ?? null;
		addModelGguf = file;
		if (file && !addModelName) {
			addModelName = autoDetectName(file.name);
		}
	}

	function handleMmprojSelect(e: Event) {
		const input = e.target as HTMLInputElement;
		addModelMmproj = input.files?.[0] ?? null;
	}

	function resetAddModel() {
		addModelOpen = false;
		addModelGguf = null;
		addModelMmproj = null;
		addModelName = '';
		addModelUploading = false;
	}

	async function handleAddModel() {
		if (!addModelGguf || addModelUploading) return;
		addModelUploading = true;
		try {
			await api.addModel(addModelGguf, addModelMmproj, addModelName);
			showNotification('Model added successfully.', 'info');
			resetAddModel();
			await loadModels();
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to add model.';
			showNotification(msg, 'error');
		} finally {
			addModelUploading = false;
		}
	}

	function isTierModel(model: ModelInfo): boolean {
		return model.tier === 'lite' || model.tier === 'standard' || model.tier === 'pro';
	}

	function startRename(model: ModelInfo) {
		editingModelId = model.id;
		editingName = model.name;
	}

	async function saveRename() {
		if (!editingModelId || !editingName.trim()) return;
		try {
			await api.renameModel(editingModelId, editingName.trim());
			await loadModels();
			editingModelId = null;
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to rename model.';
			showNotification(msg, 'error');
		}
	}

	function cancelRename() {
		editingModelId = null;
	}

	async function confirmDeleteModel(model: ModelInfo) {
		if (deleteModelConfirm !== model.id) {
			deleteModelConfirm = model.id;
			return;
		}
		try {
			await api.deleteModel(model.id);
			await loadModels();
			deleteModelConfirm = null;
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to delete model.';
			showNotification(msg, 'error');
			deleteModelConfirm = null;
		}
	}

	async function handleDeleteAll() {
		if (deleteAllStep === 0) {
			deleteAllStep = 1;
			deleteAllTimeout = setTimeout(() => { deleteAllStep = 0; }, 5000);
			return;
		}
		if (deleteAllStep === 1) {
			if (deleteAllTimeout) clearTimeout(deleteAllTimeout);
			await deleteAllConversations();
			deleteAllStep = 0;
			showNotification('All conversations deleted.', 'info');
		}
	}

	function resetInferenceDefaults() {
		if (!config) return;
		config.temperature = 0.7;
		config.top_p = 0.95;
		config.top_k = 40;
		config.repeat_penalty = 1.1;
		config.max_tokens = 0;
		config.seed = -1;
	}

	function resetSystemPrompt() {
		systemPrompt = defaultPrompt;
		systemPromptIsDefault = true;
	}

	function closeSettingsNav() {
		toggleSettings();
		if (typeof window !== 'undefined' && window.location.pathname === '/settings') {
			window.history.back();
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && getSettingsOpen()) {
			closeSettingsNav();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if getSettingsOpen()}
	<div class="settings-page">
		<div class="settings-header">
			<button class="back-btn" onclick={closeSettingsNav} aria-label="Back to chat">
				<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<polyline points="15 18 9 12 15 6" />
				</svg>
				Back
			</button>
			<h2>Settings</h2>
			<div class="header-spacer"></div>
		</div>

		{#if config}
			<nav class="tab-bar">
				<button class="tab" class:active={activeTab === 'general'} onclick={() => (activeTab = 'general')}>General</button>
				<button class="tab" class:active={activeTab === 'models'} onclick={() => (activeTab = 'models')}>Models</button>
				<button class="tab" class:active={activeTab === 'helper'} onclick={() => { activeTab = 'helper'; loadHelperModels(); }}>Helper models</button>
				{#if config.pro_mode}
					<button class="tab" class:active={activeTab === 'advanced'} onclick={() => (activeTab = 'advanced')}>Advanced</button>
				{/if}
			</nav>
		{/if}

		<div class="settings-body">
			{#if loading}
				<p class="loading">Loading settings...</p>
			{:else if error && !config}
				<div class="error-state">
					<p class="error-msg">{error}</p>
					<button class="secondary-btn" onclick={loadConfig}>Retry</button>
				</div>
			{:else if config}
				<div class="settings-content">

					{#if activeTab === 'general'}
						<!-- ==================== APPEARANCE ==================== -->
						<section class="section">
							<h4 class="section-title">Appearance</h4>

							<div class="field">
								<span class="field-label">Theme</span>
								<div class="theme-buttons">
									<button
										class="theme-btn"
										class:active={config.theme === 'light'}
										onclick={() => setTheme('light')}
									>
										<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<circle cx="12" cy="12" r="5" />
											<line x1="12" y1="1" x2="12" y2="3" />
											<line x1="12" y1="21" x2="12" y2="23" />
											<line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
											<line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
											<line x1="1" y1="12" x2="3" y2="12" />
											<line x1="21" y1="12" x2="23" y2="12" />
											<line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
											<line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
										</svg>
										Light
									</button>
									<button
										class="theme-btn"
										class:active={config.theme === 'dark'}
										onclick={() => setTheme('dark')}
									>
										<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
										</svg>
										Dark
									</button>
									<button
										class="theme-btn"
										class:active={config.theme === 'system'}
										onclick={() => setTheme('system')}
									>
										<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
											<line x1="8" y1="21" x2="16" y2="21" />
											<line x1="12" y1="17" x2="12" y2="21" />
										</svg>
										System
									</button>
								</div>
							</div>
						</section>

						<!-- ==================== ACCENT COLOR ==================== -->
						<section class="section">
							<div class="field">
								<span class="field-label">Accent Color</span>
								<div class="accent-presets">
									{#each accentPresets as preset}
										<button
											class="accent-swatch"
											class:active={(config.accent_color || '') === preset.hex}
											style="background-color: {preset.hex || '#6b7280'}"
											onclick={() => setAccentColor(preset.hex)}
											title={preset.label}
											aria-label="Accent color: {preset.label}"
										></button>
									{/each}
								</div>
								<div class="accent-custom">
									<label class="accent-custom-label" for="custom-accent">Custom</label>
									<input
										id="custom-accent"
										type="text"
										class="accent-custom-input"
										placeholder="#hex"
										value={customAccentInput}
										oninput={(e) => handleCustomAccentInput(e.currentTarget.value)}
										maxlength={7}
									/>
								</div>
							</div>
						</section>

						<!-- ==================== CHAT BEHAVIOR ==================== -->
						<section class="section">
							<h4 class="section-title">Chat Behavior</h4>

							<div class="field">
								<span class="field-label">Custom Instructions</span>
								<p class="field-desc">
									Additional instructions included with every message. For example: "Always respond in German" or "You are a coding assistant."
								</p>
								<textarea
									class="textarea"
									rows="3"
									bind:value={config.custom_instructions}
									placeholder="Enter custom instructions..."
								></textarea>
							</div>
						</section>

						<!-- ==================== PRO MODE ==================== -->
						<section class="section">
							<div class="field">
								<div class="toggle-row">
									<div>
										<span class="field-label">Pro Mode</span>
										<p class="field-desc">Show advanced settings for model parameters and server configuration.</p>
									</div>
									<button
										class="toggle"
										class:on={config.pro_mode}
										onclick={toggleProMode}
										role="switch"
										aria-checked={config.pro_mode}
										aria-label="Toggle Pro Mode"
									>
										<span class="toggle-knob"></span>
									</button>
								</div>
							</div>
						</section>

						<!-- ==================== SAVE BUTTON ==================== -->
						{#if error}
							<p class="error-msg">{error}</p>
						{/if}
						<button class="save-btn" onclick={save} disabled={saving}>
							{saving ? 'Saving...' : 'Save Settings'}
						</button>

						<!-- ==================== ABOUT & UPDATES ==================== -->
						<section class="section">
							<h4 class="section-title">About & Updates</h4>
							<div class="field">
								<p class="version-text">
									{#if appVersion}
										Fllint v{appVersion}
									{:else}
										Fllint
									{/if}
								</p>
								<p class="field-desc">Fllint checks for updates automatically when you open the app.</p>
								<div class="update-row">
									<button
										class="secondary-btn"
										onclick={handleCheckUpdate}
										disabled={checkingUpdate}
									>
										{checkingUpdate ? 'Checking...' : 'Check for Updates'}
									</button>
								</div>
							</div>
						</section>

						<!-- ==================== DANGER ZONE ==================== -->
						<section class="section danger-zone">
							<h4 class="section-title danger-title">Danger Zone</h4>

							<div class="field">
								<span class="field-label">Delete All Conversations</span>
								<p class="field-desc">
									This will permanently delete all {getConversations().length} conversation{getConversations().length !== 1 ? 's' : ''}. This cannot be undone.
								</p>
								{#if deleteAllStep === 0}
									<button
										class="danger-btn"
										onclick={handleDeleteAll}
										disabled={getConversations().length === 0}
									>
										Delete All Conversations
									</button>
								{:else}
									<button class="danger-btn confirm" onclick={handleDeleteAll}>
										Yes, delete all conversations
									</button>
								{/if}
							</div>

							<div class="field uninstall-info">
								<p class="field-desc">
									To completely remove Fllint from your system, delete the Fllint folder. All models, conversations, and settings are stored there — nothing else is left on your computer.
								</p>
							</div>
						</section>

					{:else if activeTab === 'models'}
						<!-- ==================== LOADED MODELS ==================== -->
						{#if loadedModels.length > 0}
							<section class="section">
								<h4 class="section-title">Loaded Models</h4>
								<div class="model-list">
									{#each loadedModels as model (model.id)}
										<div class="model-item loaded-item">
											<div class="model-info">
												<div class="model-name-row">
													{#if model.external}
														<span class="external-dot-settings"></span>
													{:else}
														<span class="loaded-dot-settings"></span>
													{/if}
													<span class="model-name">{model.name}</span>
													{#if model.active || config?.default_model_id === model.id}
														<span class="badge active-badge">Default</span>
													{/if}
													{#if isModelStarting(model.id)}
														<span class="badge starting-badge">Loading</span>
													{:else}
														<span class="badge loaded-badge">Loaded</span>
													{/if}
												</div>
												<div class="model-meta">
													{#if model.size}
														<span>{formatSize(model.size)}</span>
													{/if}
												</div>
											</div>
											<div class="model-actions">
												{#if !model.active && config?.default_model_id !== model.id}
													<button
														class="small-btn"
														onclick={() => setDefaultModel(model.id)}
													>
														Set Default
													</button>
												{/if}
												{#if !model.external}
													<button
														class="small-btn unload-btn"
														onclick={() => handleUnload(model.id)}
														disabled={unloadingModelId === model.id}
													>
														{unloadingModelId === model.id ? 'Unloading...' : 'Unload'}
													</button>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							</section>
						{/if}

						<!-- ==================== DEFAULT MODEL ==================== -->
						<section class="section">
							<h4 class="section-title">Default Model</h4>
							<p class="field-desc">The model loaded automatically when Fllint starts.</p>
							<select class="select" value={config?.default_model_id || ''} onchange={(e) => setDefaultModel(e.currentTarget.value)}>
								<option value="">Auto (smallest local model)</option>
								{#each getModels() as model (model.id)}
									<option value={model.id}>
										{model.name}{model.external ? ' (external)' : ''}
									</option>
								{/each}
							</select>
						</section>

						<!-- ==================== GET MODELS ==================== -->
						{#if registryModels.length > 0}
							<section class="section">
								<h4 class="section-title">Get Models</h4>
								<p class="field-desc">Download official models directly. Files are saved to your models folder.</p>

								<div class="download-list">
									{#each registryModels.filter(m => m.category !== 'helper') as model (model.id)}
										{@const dl = findActiveDownload(model.id)}
										<div class="download-card">
											<div class="model-info">
												<div class="model-name-row">
													<span class="model-name">{model.display_name}</span>
												</div>
												<div class="model-meta">
													<span>{formatSize(model.size + (model.mmproj_size ?? 0))}</span>
												</div>
											</div>
											<div class="download-action">
												{#if model.downloaded}
													<span class="download-done" title="Downloaded">
														<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
															<polyline points="20 6 9 17 4 12" />
														</svg>
													</span>
												{:else if dl?.state === 'downloading'}
													<div class="download-progress-row">
														<div class="progress-bar">
															<div class="progress-fill" style="width: {progressPercent(dl)}%"></div>
														</div>
														<span class="progress-text">{progressPercent(dl)}%</span>
														<button class="small-btn danger-text" onclick={() => handleCancelDownload(dl.id)}>Cancel</button>
													</div>
												{:else if dl?.state === 'queued'}
													<span class="queue-text">Waiting...</span>
													<button class="small-btn danger-text" onclick={() => handleCancelDownload(dl.id)}>Cancel</button>
												{:else if dl?.state === 'error'}
													<span class="error-text-small" title={dl.error}>{dl.error}</span>
													<button class="small-btn" onclick={() => handleStartDownload(model.id)}>Retry</button>
												{:else}
													<button class="secondary-btn download-btn" onclick={() => handleStartDownload(model.id)}>Download</button>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							</section>
						{/if}

						<!-- ==================== ADD CUSTOM MODEL ==================== -->
						<section class="section">
							<h4 class="section-title">Add Custom Model</h4>
							<p class="field-desc">Add a local GGUF model file. The file will be copied to your models folder.</p>

							{#if !addModelOpen}
								<button class="secondary-btn" onclick={() => (addModelOpen = true)}>Add Model</button>
							{:else}
								<div class="add-model-form">
									<div class="field">
										<label class="field-label" for="add-gguf">Model File (.gguf) *</label>
										<input id="add-gguf" type="file" accept=".gguf" onchange={handleGgufSelect} class="file-input" />
										{#if addModelGguf}
											<p class="field-hint">{addModelGguf.name} — {formatSize(addModelGguf.size)}</p>
										{/if}
									</div>

									<div class="field">
										<label class="field-label" for="add-mmproj">Vision Projection (.gguf) — optional</label>
										<input id="add-mmproj" type="file" accept=".gguf" onchange={handleMmprojSelect} class="file-input" />
										{#if addModelMmproj}
											<p class="field-hint">{addModelMmproj.name} — {formatSize(addModelMmproj.size)}</p>
										{/if}
									</div>

									<div class="field">
										<label class="field-label" for="add-name">Display Name</label>
										<input
											id="add-name"
											type="text"
											class="input"
											bind:value={addModelName}
											placeholder="Auto-detected from filename"
										/>
									</div>

									<div class="add-model-actions">
										<button class="secondary-btn" onclick={resetAddModel} disabled={addModelUploading}>Cancel</button>
										<button
											class="primary-btn"
											onclick={handleAddModel}
											disabled={!addModelGguf || addModelUploading}
										>
											{addModelUploading ? 'Copying...' : 'Add Model'}
										</button>
									</div>
								</div>
							{/if}
						</section>

						<!-- ==================== MODEL PROVIDERS ==================== -->
						<section class="section">
							<h4 class="section-title">Model Providers</h4>
							<p class="field-desc">Connect to external model servers like Ollama.</p>

							{#each allProviders as prov (prov.id)}
								<div class="provider-card">
									<div class="provider-header">
										<div class="provider-info">
											<span class="external-dot-settings"></span>
											<div>
												<span class="provider-name">{prov.name}</span>
												<span class="provider-url">{prov.base_url}</span>
											</div>
										</div>
										<div class="provider-actions">
											<button
												class="toggle small-toggle"
												class:on={prov.enabled}
												onclick={() => toggleProviderEnabled(prov)}
												role="switch"
												aria-checked={prov.enabled}
												title={prov.enabled ? 'Disable' : 'Enable'}
											>
												<span class="toggle-knob"></span>
											</button>
											<button class="small-btn muted" onclick={() => startEditProvider(prov)}>Edit</button>
											<button
												class="small-btn danger-text"
												onclick={() => deleteProvider(prov.id)}
											>
												{deletingProviderId === prov.id ? 'Confirm?' : 'Delete'}
											</button>
										</div>
									</div>
									<div class="provider-meta">
										<span>{prov.models.length} model{prov.models.length !== 1 ? 's' : ''} selected</span>
										<button class="small-btn" onclick={() => toggleManageModels(prov)}>
											{managingModelsId === prov.id ? 'Hide Models' : 'Manage Models'}
										</button>
									</div>

									{#if managingModelsId === prov.id}
										<div class="provider-models">
											{#if fetchingModels === prov.id}
												<p class="field-desc">Fetching models...</p>
											{:else if availableModels.length === 0}
												<p class="field-desc">No models found on this server.</p>
											{:else}
												<div class="model-checklist">
													{#each availableModels as m (m.name)}
														<div class="model-check-item">
															<div class="model-check-header">
																<span class="model-check-name">{m.name}</span>
																{#if m.details?.parameter_size}
																	<span class="model-check-size">{m.details.parameter_size}</span>
																{/if}
															</div>
															<div class="model-role-row">
																{#each MODEL_ROLES as role}
																	<label class="role-chip" class:active={isModelRoleSelected(m.name, role.id)}>
																		<input
																			type="checkbox"
																			checked={isModelRoleSelected(m.name, role.id)}
																			onchange={() => toggleModelRole(m.name, role.id)}
																		/>
																		{role.label}
																	</label>
																{/each}
															</div>
														</div>
													{/each}
												</div>
												<p class="field-desc" style="margin-top: 8px;">Assign each model to one or more roles. Models not assigned to any role will not appear in selectors.</p>
												<button
													class="secondary-btn"
													onclick={() => saveProviderModels(prov.id)}
													disabled={savingModels}
													style="margin-top: 8px;"
												>
													{savingModels ? 'Saving...' : 'Save Selection'}
												</button>
											{/if}
										</div>
									{/if}
								</div>
							{/each}

							{#if editingProviderId}
								{@const editProv = allProviders.find((p) => p.id === editingProviderId)}
								{@const typeInfo = getProviderTypeInfo(providerForm.type)}
								<div class="provider-form">
									<h5 class="form-title">Edit Provider</h5>
									<div class="field">
										<span class="field-label">Name</span>
										<input class="text-input" bind:value={providerForm.name} placeholder="My Ollama" />
									</div>
									<div class="field">
										<span class="field-label">URL</span>
										<input class="text-input" bind:value={providerForm.base_url} placeholder="http://localhost:11434" />
									</div>
									{#if typeInfo?.requires_key}
										<div class="field">
											<span class="field-label">API Key</span>
											<input class="text-input" type="password" bind:value={providerForm.api_key} placeholder={editProv?.has_api_key ? '(unchanged)' : 'Enter API key'} />
											<p class="field-desc warning-text">API keys are stored unencrypted in your Fllint Data folder. Use unique keys with spending limits. Only trusted users should access the Fllint folder.</p>
										</div>
									{/if}
									{#if testResult === 'success'}
										<p class="success-text">Connection successful.</p>
									{:else if testResult === 'error'}
										<p class="error-text-small">{testError}</p>
									{/if}
									<div class="button-row">
										{#if editProv}
											<button class="secondary-btn" onclick={() => testProviderConnection(editProv.id)} disabled={testingConnection}>
												{testingConnection ? 'Testing...' : 'Test Connection'}
											</button>
										{/if}
										<button class="secondary-btn" onclick={saveProvider}>Save</button>
										<button class="small-btn muted" onclick={resetProviderForm}>Cancel</button>
									</div>
								</div>
							{:else if addingProvider}
								{@const typeInfo = getProviderTypeInfo(providerForm.type)}
								<div class="provider-form">
									<h5 class="form-title">Add Provider</h5>
									<div class="field">
										<span class="field-label">Type</span>
										<select class="select" bind:value={providerForm.type} onchange={handleProviderTypeChange}>
											{#each providerTypes as pt (pt.type)}
												<option value={pt.type}>{pt.label}</option>
											{/each}
										</select>
									</div>
									<div class="field">
										<span class="field-label">Name</span>
										<input class="text-input" bind:value={providerForm.name} placeholder="My Ollama" />
									</div>
									<div class="field">
										<span class="field-label">URL</span>
										<input class="text-input" bind:value={providerForm.base_url} placeholder="http://localhost:11434" />
									</div>
									{#if typeInfo?.requires_key}
										<div class="field">
											<span class="field-label">API Key</span>
											<input class="text-input" type="password" bind:value={providerForm.api_key} placeholder="Enter API key" />
											<p class="field-desc warning-text">API keys are stored unencrypted in your Fllint Data folder. Use unique keys with spending limits. Only trusted users should access the Fllint folder.</p>
										</div>
									{/if}
									<div class="button-row">
										<button class="secondary-btn" onclick={saveProvider}>Save</button>
										<button class="small-btn muted" onclick={resetProviderForm}>Cancel</button>
									</div>
								</div>
							{:else}
								<button class="secondary-btn" onclick={startAddProvider} style="margin-top: 8px;">
									+ Add Provider
								</button>
							{/if}
						</section>

						<!-- ==================== MODEL SELECTOR ORDER ==================== -->
						<section class="section">
							<h4 class="section-title">Model Selector Order</h4>
							<p class="field-desc">Drag to reorder pinned models. Click the pin icon to show or hide models in the selector.</p>

							{#if getModels().length === 0}
								<p class="field-desc">No models found.</p>
							{:else}
								<div class="model-list">
									{#each pinnedModels as model (model.id)}
										<!-- svelte-ignore a11y_no_static_element_interactions -->
									<div
											class="model-item draggable"
											class:drag-over={dragOverModelId === model.id}
											draggable="true"
											ondragstart={() => handleDragStart(model.id)}
											ondragover={(e) => handleDragOver(e, model.id)}
											ondragleave={handleDragLeave}
											ondrop={(e) => handleDrop(e, model.id)}
											ondragend={handleDragEnd}
										>
											<div class="drag-handle" title="Drag to reorder">
												<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
													<line x1="4" y1="6" x2="20" y2="6" /><line x1="4" y1="12" x2="20" y2="12" /><line x1="4" y1="18" x2="20" y2="18" />
												</svg>
											</div>
											<div class="model-info">
												{#if editingModelId === model.id}
													<div class="rename-row">
														<input
															class="rename-input"
															bind:value={editingName}
															onkeydown={(e) => {
																if (e.key === 'Enter') saveRename();
																if (e.key === 'Escape') cancelRename();
															}}
														/>
														<button class="small-btn" onclick={saveRename}>Save</button>
														<button class="small-btn muted" onclick={cancelRename}>Cancel</button>
													</div>
												{:else}
													<div class="model-name-row">
														<span class="model-name">{model.name}</span>
														{#if model.vision}
															<span class="badge vision-badge" title="Supports image input">Vision</span>
														{/if}
													</div>
													<div class="model-meta">
														{#if model.size}
															<span>{formatSize(model.size)}</span>
														{/if}
													</div>
												{/if}
											</div>
											{#if editingModelId !== model.id}
												<div class="model-actions">
													<button
														class="small-btn pin-btn pinned"
														onclick={() => togglePin(model)}
														title="Unpin from selector"
													>
														<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
															<path d="M12 17v5M6.7 3.5l3.6 3.6L7.5 9.9l6.6 6.6 2.8-2.8 3.6 3.6" />
															<path d="M17.3 3.5L20.5 6.7" />
														</svg>
													</button>
													{#if !isTierModel(model)}
														<button class="small-btn muted" onclick={() => startRename(model)} title="Rename">
															Rename
														</button>
													{/if}
													{#if !model.active && !model.loaded}
														<button
															class="small-btn danger-text"
															onclick={() => confirmDeleteModel(model)}
														>
															{deleteModelConfirm === model.id ? 'Confirm?' : 'Delete'}
														</button>
													{/if}
												</div>
											{/if}
										</div>
									{/each}
								</div>

								{#if unpinnedModels.length > 0}
									<p class="other-models-label">Other models</p>
									<div class="model-list">
										{#each unpinnedModels as model (model.id)}
											<div class="model-item">
												<div class="model-info">
													{#if editingModelId === model.id}
														<div class="rename-row">
															<input
																class="rename-input"
																bind:value={editingName}
																onkeydown={(e) => {
																	if (e.key === 'Enter') saveRename();
																	if (e.key === 'Escape') cancelRename();
																}}
															/>
															<button class="small-btn" onclick={saveRename}>Save</button>
															<button class="small-btn muted" onclick={cancelRename}>Cancel</button>
														</div>
													{:else}
														<div class="model-name-row">
															<span class="model-name">{model.name}</span>
															{#if model.vision}
																<span class="badge vision-badge" title="Supports image input">Vision</span>
															{/if}
														</div>
														<div class="model-meta">
															{#if model.size}
																<span>{formatSize(model.size)}</span>
															{/if}
														</div>
													{/if}
												</div>
												{#if editingModelId !== model.id}
													<div class="model-actions">
														<button
															class="small-btn pin-btn"
															onclick={() => togglePin(model)}
															title="Pin to selector"
														>
															<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
																<path d="M12 17v5M6.7 3.5l3.6 3.6L7.5 9.9l6.6 6.6 2.8-2.8 3.6 3.6" />
																<path d="M17.3 3.5L20.5 6.7" />
															</svg>
														</button>
														<button class="small-btn muted" onclick={() => startRename(model)} title="Rename">
															Rename
														</button>
														{#if !model.active && !model.loaded}
															<button
																class="small-btn danger-text"
																onclick={() => confirmDeleteModel(model)}
															>
																{deleteModelConfirm === model.id ? 'Confirm?' : 'Delete'}
															</button>
														{/if}
													</div>
												{/if}
											</div>
										{/each}
									</div>
								{/if}
							{/if}

							<div class="button-row" style="margin-top: 12px;">
								<button class="secondary-btn" onclick={() => api.openFolder('models')}>
									Open Models Folder
								</button>
								<button class="secondary-btn" onclick={() => api.refreshModels().then(() => loadModels())}>
									Refresh
								</button>
							</div>
						</section>

					{:else if activeTab === 'helper'}
						<!-- ==================== HELPER MODELS ==================== -->

						{#if helperLoading}
							<p class="loading">Loading helper models...</p>
						{:else}
							{#each helperSlots as slot}
								<section class="section">
									<h4 class="section-title">
										{slot.slot} Model
										{#if slot.enabled}
											<span class="active-badge">Active</span>
										{:else}
											<span class="coming-soon-badge">Coming soon</span>
										{/if}
									</h4>

									{#if slot.slot === 'Summary'}
										<p class="section-description">
											Generates conversation titles from your first message. A small, fast model works best.
										</p>

										{@const summaryReg = registryModels.find(m => m.id === 'helper-summary-qwen3.5-0.8b')}
										{#if summaryReg && !summaryReg.downloaded}
											{@const dl = findActiveDownload(summaryReg.id)}
											<div class="download-list" style="margin-bottom: 12px;">
												<div class="download-card">
													<div class="model-info">
														<div class="model-name-row">
															<span class="model-name">{summaryReg.display_name}</span>
														</div>
														<div class="model-meta">
															<span>{formatSize(summaryReg.size)}</span>
														</div>
													</div>
													<div class="download-action">
														{#if dl?.state === 'downloading'}
															<div class="download-progress-row">
																<div class="progress-bar">
																	<div class="progress-fill" style="width: {progressPercent(dl)}%"></div>
																</div>
																<span class="progress-text">{progressPercent(dl)}%</span>
																<button class="small-btn danger-text" onclick={() => handleCancelDownload(dl.id)}>Cancel</button>
															</div>
														{:else if dl?.state === 'queued'}
															<span class="queue-text">Waiting...</span>
															<button class="small-btn danger-text" onclick={() => handleCancelDownload(dl.id)}>Cancel</button>
														{:else if dl?.state === 'error'}
															<span class="error-text-small" title={dl.error}>{dl.error}</span>
															<button class="small-btn" onclick={() => handleStartDownload(summaryReg.id)}>Retry</button>
														{:else}
															<button class="secondary-btn download-btn" onclick={() => handleStartDownload(summaryReg.id)}>Download</button>
														{/if}
													</div>
												</div>
											</div>
										{/if}

										<div class="field">
											<label class="field-label" for="summary-model">Model</label>
											<select
												id="summary-model"
												class="select"
												value={slot.configured_model_id || ''}
												onchange={(e) => {
													const target = e.target as HTMLSelectElement;
													saveHelperConfig({ summary_model_id: target.value || undefined });
													// Update local state optimistically
													slot.configured_model_id = target.value;
												}}
												disabled={savingHelper}
											>
												<option value="">None (use text truncation)</option>
												{#each slot.available_models as model}
													<option value={model.id}>
														{model.name}{model.external ? ' (External)' : ''}{model.size ? ` — ${formatSize(model.size)}` : ''}
													</option>
												{/each}
											</select>
										</div>

									{:else if slot.slot === 'OCR'}
										<p class="section-description">
											Extracts text from scanned PDFs using a vision model. Click the pen icon on PDF attachments to start OCR.
										</p>

										{@const ocrReg = registryModels.find(m => m.id === 'helper-ocr-glm-ocr')}
										{#if ocrReg && !ocrReg.downloaded}
											{@const dl = findActiveDownload(ocrReg.id)}
											<div class="download-list" style="margin-bottom: 12px;">
												<div class="download-card">
													<div class="model-info">
														<div class="model-name-row">
															<span class="model-name">{ocrReg.display_name}</span>
														</div>
														<div class="model-meta">
															<span>{formatSize(ocrReg.size + (ocrReg.mmproj_size || 0))}</span>
														</div>
													</div>
													<div class="download-action">
														{#if dl?.state === 'downloading'}
															<div class="download-progress-row">
																<div class="progress-bar">
																	<div class="progress-fill" style="width: {progressPercent(dl)}%"></div>
																</div>
																<span class="progress-text">{progressPercent(dl)}%</span>
																<button class="small-btn danger-text" onclick={() => handleCancelDownload(dl.id)}>Cancel</button>
															</div>
														{:else if dl?.state === 'queued'}
															<span class="queue-text">Waiting...</span>
															<button class="small-btn danger-text" onclick={() => handleCancelDownload(dl.id)}>Cancel</button>
														{:else if dl?.state === 'error'}
															<span class="error-text-small" title={dl.error}>{dl.error}</span>
															<button class="small-btn" onclick={() => handleStartDownload(ocrReg.id)}>Retry</button>
														{:else}
															<button class="secondary-btn download-btn" onclick={() => handleStartDownload(ocrReg.id)}>Download</button>
														{/if}
													</div>
												</div>
											</div>
										{/if}

										<div class="field">
											<label class="field-label" for="ocr-model">Model</label>
											<select
												id="ocr-model"
												class="select"
												value={slot.configured_model_id || ''}
												onchange={(e) => {
													const target = e.target as HTMLSelectElement;
													saveHelperConfig({ ocr_model_id: target.value });
													slot.configured_model_id = target.value;
													setOcrEnabled(!!target.value);
												}}
												disabled={savingHelper}
											>
												<option value="">None (OCR disabled)</option>
												{#each slot.available_models as model}
													<option value={model.id}>
														{model.name}{model.external ? ' (External)' : ''}{model.size ? ` — ${formatSize(model.size)}` : ''}
													</option>
												{/each}
											</select>
										</div>
									{:else if slot.slot === 'Embedding'}
										<p class="section-description">
											Creates vector representations for semantic search across conversations.
										</p>
										<p class="coming-soon-text">Embedding model support will be available in a future update.</p>
									{/if}
								</section>
							{/each}

							{#if helperSlots.length === 0}
								<section class="section">
									<p class="section-description">No helper model slots available.</p>
								</section>
							{/if}

							<section class="section">
								<p class="section-description">
									To use external models for helper tasks, assign them the appropriate role in
									<button class="link-btn" onclick={() => { activeTab = 'models'; }}>Model Providers</button>.
								</p>
							</section>
						{/if}

					{:else if activeTab === 'advanced'}
						<!-- ==================== SYSTEM PROMPT ==================== -->
						<section class="section">
							<h4 class="section-title">
								System Prompt
								{#if !systemPromptIsDefault}
									<span class="modified-badge">Modified</span>
								{/if}
							</h4>
							<textarea
								class="textarea system-prompt-textarea"
								rows="6"
								bind:value={systemPrompt}
								placeholder={defaultPrompt}
								oninput={() => { systemPromptIsDefault = systemPrompt === defaultPrompt; }}
							></textarea>
							<div class="button-row" style="margin-top: 8px;">
								<button class="small-btn muted" onclick={resetSystemPrompt}>
									Reset to Default
								</button>
							</div>
						</section>

						<!-- ==================== INFERENCE PARAMETERS ==================== -->
						<section class="section">
							<h4 class="section-title">Inference Parameters</h4>

							<div class="field">
								<div class="slider-header">
									<span class="field-label">Temperature</span>
									<span class="slider-value">{config.temperature.toFixed(2)}</span>
								</div>
								<input type="range" class="slider" min="0" max="2" step="0.05" bind:value={config.temperature} />
							</div>

							<div class="field">
								<div class="slider-header">
									<span class="field-label">Top P</span>
									<span class="slider-value">{config.top_p.toFixed(2)}</span>
								</div>
								<input type="range" class="slider" min="0" max="1" step="0.05" bind:value={config.top_p} />
							</div>

							<div class="field">
								<span class="field-label">Top K</span>
								<input type="number" class="number-input" min="0" max="200" bind:value={config.top_k} />
							</div>

							<div class="field">
								<div class="slider-header">
									<span class="field-label">Repeat Penalty</span>
									<span class="slider-value">{config.repeat_penalty.toFixed(2)}</span>
								</div>
								<input type="range" class="slider" min="0" max="2" step="0.05" bind:value={config.repeat_penalty} />
							</div>

							<div class="field">
								<span class="field-label">Max Tokens</span>
								<p class="field-desc">0 = no limit</p>
								<input type="number" class="number-input" min="0" max="32768" bind:value={config.max_tokens} />
							</div>

							<div class="field">
								<span class="field-label">Seed</span>
								<p class="field-desc">-1 = random</p>
								<input type="number" class="number-input" min="-1" max="999999" bind:value={config.seed} />
							</div>

							<div class="button-row">
								<button class="secondary-btn" onclick={resetInferenceDefaults}>
									Reset to Defaults
								</button>
							</div>
						</section>

						<!-- ==================== FORWARD PARAMS TO EXTERNAL ==================== -->
						<section class="section">
							<div class="field">
								<div class="toggle-row">
									<div>
										<span class="field-label">Forward to External Models</span>
										<p class="field-desc">Send these inference parameters to external model providers too. When off, external servers use their own defaults.</p>
									</div>
									<button
										class="toggle"
										class:on={config.forward_params_to_external}
										onclick={() => { if (config) config.forward_params_to_external = !config.forward_params_to_external; }}
										role="switch"
										aria-checked={config.forward_params_to_external}
										aria-label="Forward inference params to external models"
									>
										<span class="toggle-knob"></span>
									</button>
								</div>
							</div>
						</section>

						<!-- ==================== SERVER CONFIG ==================== -->
						<section class="section">
							<h4 class="section-title">Server Configuration</h4>

							<p class="field-desc note">Changes to these settings take effect when the model is next loaded.</p>

							<div class="field">
								<span class="field-label">Context Size</span>
								<select class="select" bind:value={config.ctx_size}>
									<option value={2048}>2,048</option>
									<option value={4096}>4,096</option>
									<option value={8192}>8,192</option>
									<option value={16384}>16,384</option>
									<option value={32768}>32,768</option>
								</select>
							</div>

							<div class="field">
								<span class="field-label">Response Buffer</span>
								<p class="field-desc">Tokens reserved for the model's response. Prevents sending when context is nearly full.</p>
								<input type="number" class="number-input" min="256" max="8192" step="256" bind:value={config.response_buffer} />
							</div>

							<div class="field">
								<span class="field-label">GPU Layers</span>
								<p class="field-desc">Number of layers offloaded to GPU. 999 = auto (all layers).</p>
								<input type="number" class="number-input" min="0" max="999" bind:value={config.n_gpu_layers} />
							</div>

							<div class="field">
								<span class="field-label">Flash Attention</span>
								<select class="select" bind:value={config.flash_attn}>
									<option value="auto">Auto</option>
									<option value="on">On</option>
									<option value="off">Off</option>
								</select>
							</div>

							<div class="field">
								<span class="field-label">Port</span>
								<p class="field-desc">Requires app restart to take effect.</p>
								<input type="number" class="number-input" min="1024" max="65535" bind:value={config.port} />
							</div>

							<div class="button-row">
								<button class="secondary-btn" onclick={() => api.openFolder('data')}>
									Open Data Folder
								</button>
							</div>
						</section>

						<!-- ==================== SAVE BUTTON ==================== -->
						{#if error}
							<p class="error-msg">{error}</p>
						{/if}
						<button class="save-btn" onclick={save} disabled={saving}>
							{saving ? 'Saving...' : 'Save Settings'}
						</button>
					{/if}

				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.settings-page {
		position: fixed;
		inset: 0;
		background: var(--bg-primary);
		z-index: 100;
		display: flex;
		flex-direction: column;
	}

	.settings-header {
		display: flex;
		align-items: center;
		padding: 0 20px;
		height: var(--header-height);
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}

	.settings-header h2 {
		font-size: 1.05rem;
		font-weight: 600;
		color: var(--text-primary);
	}

	.header-spacer {
		flex: 1;
	}

	.back-btn {
		display: flex;
		align-items: center;
		gap: 4px;
		padding: 6px 10px;
		border-radius: var(--radius);
		color: var(--text-secondary);
		font-size: 0.9rem;
		margin-right: 12px;
		transition: all var(--transition);
	}

	.back-btn:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	/* Tab bar */
	.tab-bar {
		display: flex;
		gap: 0;
		padding: 0 24px;
		max-width: calc(560px + 48px);
		margin: 0 auto;
		width: 100%;
		border-bottom: 1px solid var(--border-light);
		flex-shrink: 0;
	}

	.tab {
		padding: 10px 16px;
		font-size: 0.85rem;
		font-weight: 500;
		color: var(--text-muted);
		border-bottom: 2px solid transparent;
		margin-bottom: -1px;
		transition: all var(--transition);
	}

	.tab:hover {
		color: var(--text-secondary);
	}

	.tab.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
	}

	.settings-body {
		flex: 1;
		overflow-y: auto;
		display: flex;
		justify-content: center;
	}

	.settings-content {
		width: 100%;
		max-width: 560px;
		padding: 28px 24px 48px;
	}

	.loading {
		color: var(--text-muted);
		text-align: center;
		padding: 60px 20px;
	}

	.error-state {
		text-align: center;
		padding: 60px 20px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
	}

	/* Sections */
	.section {
		margin-bottom: 28px;
		padding-bottom: 24px;
		border-bottom: 1px solid var(--border-light);
	}

	.section:last-child {
		border-bottom: none;
		margin-bottom: 0;
	}

	.section-title {
		font-size: 0.9rem;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 16px;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	/* Fields */
	.field {
		margin-bottom: 16px;
	}

	.field:last-child {
		margin-bottom: 0;
	}

	.field-label {
		font-size: 0.85rem;
		font-weight: 500;
		color: var(--text-secondary);
		display: block;
		margin-bottom: 4px;
	}

	.field-desc {
		font-size: 0.8rem;
		color: var(--text-muted);
		margin-bottom: 8px;
		line-height: 1.4;
	}

	.note {
		padding: 8px 12px;
		background: var(--bg-secondary);
		border-radius: var(--radius);
		margin-bottom: 16px;
	}

	/* Theme buttons */
	.theme-buttons {
		display: flex;
		gap: 8px;
	}

	.theme-btn {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		padding: 8px 12px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		font-size: 0.85rem;
		color: var(--text-secondary);
		transition: all var(--transition);
		background: var(--bg-primary);
	}

	.theme-btn:hover {
		border-color: var(--text-muted);
		color: var(--text-primary);
	}

	.theme-btn.active {
		border-color: var(--accent);
		color: var(--accent);
		background: var(--accent-light);
	}

	/* Accent Color */
	.accent-presets {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		margin-bottom: 8px;
	}

	.accent-swatch {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		border: 2px solid transparent;
		cursor: pointer;
		transition: border-color var(--transition), transform var(--transition);
		padding: 0;
	}

	.accent-swatch:hover {
		transform: scale(1.1);
	}

	.accent-swatch.active {
		border-color: var(--text-primary);
		box-shadow: 0 0 0 2px var(--bg-primary), 0 0 0 4px var(--text-secondary);
	}

	.accent-custom {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.accent-custom-label {
		font-size: 0.8125rem;
		color: var(--text-secondary);
	}

	.accent-custom-input {
		width: 90px;
		padding: 4px 8px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-input);
		color: var(--text-primary);
		font-size: 0.8125rem;
		font-family: monospace;
	}

	.accent-custom-input::placeholder {
		color: var(--text-muted);
	}

	/* Toggle */
	.toggle-row {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
	}

	.version-text {
		color: var(--text-muted);
		font-size: 0.85rem;
		margin: 0 0 4px;
	}

	.update-row {
		margin-top: 12px;
	}

	.toggle {
		width: 44px;
		height: 24px;
		border-radius: 12px;
		background: var(--border);
		position: relative;
		transition: background var(--transition);
		flex-shrink: 0;
		margin-top: 2px;
	}

	.toggle.on {
		background: var(--accent);
	}

	.toggle-knob {
		position: absolute;
		top: 2px;
		left: 2px;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: white;
		transition: transform var(--transition);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
	}

	.toggle.on .toggle-knob {
		transform: translateX(20px);
	}

	/* Model list */
	.model-list {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.model-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 12px;
		border-radius: var(--radius);
		transition: background var(--transition);
	}

	.model-item:hover {
		background: var(--bg-hover);
	}

	.model-item.loaded-item {
		background: var(--accent-light);
	}

	.model-item.draggable {
		cursor: grab;
	}

	.model-item.draggable:active {
		cursor: grabbing;
	}

	.model-item.drag-over {
		border-top: 2px solid var(--accent);
		padding-top: 8px;
	}

	.drag-handle {
		display: flex;
		align-items: center;
		padding: 4px;
		margin-right: 8px;
		color: var(--text-muted);
		cursor: grab;
		flex-shrink: 0;
	}

	.drag-handle:active {
		cursor: grabbing;
	}

	.loaded-dot-settings {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--accent);
		flex-shrink: 0;
	}

	.model-info {
		flex: 1;
		min-width: 0;
	}

	.model-name-row {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.model-name {
		font-size: 0.9rem;
		font-weight: 500;
		color: var(--text-primary);
	}

	.model-meta {
		font-size: 0.75rem;
		color: var(--text-muted);
		margin-top: 2px;
	}

	.model-actions {
		display: flex;
		gap: 4px;
		flex-shrink: 0;
		margin-left: 8px;
	}

	.other-models-label {
		margin-top: 16px;
		margin-bottom: 8px;
		font-size: 0.8rem;
		font-weight: 500;
		color: var(--text-secondary);
	}

	.badge {
		font-size: 0.65rem;
		padding: 1px 6px;
		border-radius: 4px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	.active-badge {
		background: var(--accent);
		color: white;
	}

	.loaded-badge {
		background: #d1fae5;
		color: #065f46;
	}

	:global([data-theme='dark']) .loaded-badge {
		background: #064e3b;
		color: #6ee7b7;
	}

	.starting-badge {
		background: #fef3c7;
		color: #92400e;
		animation: pulse-badge 1.5s ease-in-out infinite;
	}

	@keyframes pulse-badge {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.6; }
	}

	:global([data-theme='dark']) .starting-badge {
		background: #451a03;
		color: #fcd34d;
	}

	.vision-badge {
		background: #fef3c7;
		color: #92400e;
	}

	:global([data-theme='dark']) .vision-badge {
		background: #451a03;
		color: #fcd34d;
	}

	.rename-row {
		display: flex;
		gap: 6px;
		align-items: center;
	}

	.rename-input {
		flex: 1;
		padding: 4px 8px;
		border: 1px solid var(--accent);
		border-radius: 4px;
		background: var(--bg-primary);
		font-size: 0.85rem;
		outline: none;
	}

	/* Buttons */
	.small-btn {
		font-size: 0.75rem;
		padding: 3px 8px;
		border-radius: 4px;
		color: var(--text-secondary);
		transition: all var(--transition);
	}

	.small-btn:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.small-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.small-btn.muted {
		color: var(--text-muted);
	}

	.small-btn.danger-text {
		color: var(--danger, #dc2626);
	}

	.small-btn.danger-text:hover {
		background: var(--danger-bg, #fef2f2);
	}

	.pin-btn {
		color: var(--text-muted);
		padding: 3px 5px;
	}

	.pin-btn.pinned {
		color: var(--accent);
	}

	.pin-btn:hover {
		color: var(--accent);
		background: var(--accent-light);
	}

	.unload-btn {
		color: var(--text-secondary);
		border: 1px solid var(--border);
		padding: 3px 10px;
		border-radius: var(--radius);
	}

	.unload-btn:hover:not(:disabled) {
		border-color: var(--danger, #dc2626);
		color: var(--danger, #dc2626);
		background: var(--danger-bg, #fef2f2);
	}

	.button-row {
		display: flex;
		gap: 8px;
		margin-top: 12px;
	}

	.secondary-btn {
		padding: 8px 14px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		font-size: 0.85rem;
		color: var(--text-secondary);
		background: var(--bg-primary);
		transition: all var(--transition);
	}

	.secondary-btn:hover {
		border-color: var(--text-muted);
		color: var(--text-primary);
		background: var(--bg-hover);
	}

	/* Textarea */
	.textarea {
		width: 100%;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-input);
		font-size: 0.85rem;
		line-height: 1.5;
		resize: vertical;
		outline: none;
		transition: border-color var(--transition);
	}

	.textarea:focus {
		border-color: var(--accent);
		box-shadow: 0 0 0 3px var(--accent-light);
	}

	.system-prompt-textarea {
		font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
		font-size: 0.8rem;
	}

	.modified-badge {
		font-size: 0.7rem;
		padding: 1px 6px;
		border-radius: 4px;
		background: var(--accent-light);
		color: var(--accent);
		margin-left: 6px;
		font-weight: 600;
		text-transform: none;
		letter-spacing: normal;
	}

	.coming-soon-badge {
		font-size: 0.65rem;
		padding: 1px 6px;
		border-radius: 4px;
		background: var(--border);
		color: var(--text-secondary);
		margin-left: 6px;
		font-weight: 600;
		text-transform: none;
		letter-spacing: normal;
	}

	.section-description {
		font-size: 0.85rem;
		color: var(--text-secondary);
		margin: 0 0 12px 0;
		line-height: 1.4;
	}

	.coming-soon-text {
		font-size: 0.85rem;
		color: var(--text-tertiary, var(--text-secondary));
		font-style: italic;
		margin: 0;
	}

	/* Sliders */
	.slider-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.slider-value {
		font-size: 0.8rem;
		font-weight: 600;
		color: var(--accent);
		font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
	}

	.slider {
		width: 100%;
		height: 4px;
		-webkit-appearance: none;
		appearance: none;
		background: var(--border);
		border-radius: 2px;
		outline: none;
		margin-top: 4px;
	}

	.slider::-webkit-slider-thumb {
		-webkit-appearance: none;
		width: 16px;
		height: 16px;
		border-radius: 50%;
		background: var(--accent);
		cursor: pointer;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
	}

	/* Number input */
	.number-input {
		padding: 8px 12px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-input);
		font-size: 0.85rem;
		width: 120px;
		outline: none;
		transition: border-color var(--transition);
	}

	.number-input:focus {
		border-color: var(--accent);
		box-shadow: 0 0 0 3px var(--accent-light);
	}

	/* Select */
	.select {
		padding: 8px 12px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-input);
		font-size: 0.85rem;
		width: 100%;
		max-width: 200px;
		outline: none;
		transition: border-color var(--transition);
		cursor: pointer;
	}

	.select:focus {
		border-color: var(--accent);
		box-shadow: 0 0 0 3px var(--accent-light);
	}

	/* Save button */
	.save-btn {
		width: 100%;
		padding: 10px 20px;
		border-radius: var(--radius);
		background: var(--accent);
		color: white;
		font-weight: 600;
		font-size: 0.9rem;
		transition: background var(--transition);
		margin-bottom: 28px;
	}

	.save-btn:hover {
		background: var(--accent-hover);
	}

	.save-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.error-msg {
		color: var(--error-text);
		font-size: 0.85rem;
		padding: 8px 12px;
		border-radius: var(--radius);
		background: var(--error-bg);
		border: 1px solid var(--error-border);
		margin-bottom: 12px;
	}

	/* Danger Zone */
	.danger-zone {
		border: 1px solid var(--danger-border, #fecaca);
		border-radius: var(--radius);
		padding: 20px;
		margin-top: 8px;
		background: transparent;
	}

	.danger-title {
		color: var(--danger, #dc2626);
	}

	.danger-btn {
		padding: 8px 16px;
		border-radius: var(--radius);
		border: 1px solid var(--danger, #dc2626);
		color: var(--danger, #dc2626);
		font-size: 0.85rem;
		font-weight: 500;
		background: transparent;
		transition: all var(--transition);
	}

	.danger-btn:hover {
		background: var(--danger-bg, #fef2f2);
	}

	.danger-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.danger-btn.confirm {
		background: var(--danger, #dc2626);
		color: white;
		border-color: var(--danger, #dc2626);
	}

	.danger-btn.confirm:hover {
		background: var(--danger-hover, #b91c1c);
		border-color: var(--danger-hover, #b91c1c);
	}

	.uninstall-info {
		margin-top: 16px;
		padding-top: 16px;
		border-top: 1px solid var(--danger-border, #fecaca);
	}

	/* Download cards */
	.download-list {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.download-card {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 12px;
		border-radius: var(--radius);
		transition: background var(--transition);
	}

	.download-card:hover {
		background: var(--bg-hover);
	}

	.download-action {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
		margin-left: 12px;
	}

	.download-progress-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.progress-bar {
		width: 80px;
		height: 6px;
		background: var(--border);
		border-radius: 3px;
		overflow: hidden;
	}

	.progress-fill {
		height: 100%;
		background: var(--accent);
		border-radius: 3px;
		transition: width 0.3s ease;
	}

	.progress-text {
		font-size: 0.75rem;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
		min-width: 32px;
	}

	.download-done {
		color: #059669;
		display: flex;
		align-items: center;
	}

	:global([data-theme='dark']) .download-done {
		color: #34d399;
	}

	.queue-text {
		font-size: 0.8rem;
		color: var(--text-muted);
	}

	.error-text-small {
		font-size: 0.75rem;
		color: var(--danger, #dc2626);
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.download-btn {
		padding: 5px 12px;
		font-size: 0.8rem;
	}

	/* --- Add model styles --- */
	.add-model-form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.file-input {
		font-size: 0.85rem;
		color: var(--text-primary);
	}

	.file-input::file-selector-button {
		padding: 5px 12px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-secondary);
		color: var(--text-primary);
		font-size: 0.8rem;
		cursor: pointer;
		margin-right: 8px;
		transition: all var(--transition);
	}

	.file-input::file-selector-button:hover {
		background: var(--bg-hover);
	}

	.add-model-actions {
		display: flex;
		gap: 8px;
		justify-content: flex-end;
	}

	.primary-btn {
		padding: 6px 16px;
		border-radius: var(--radius);
		font-size: 0.85rem;
		background: var(--accent);
		color: white;
		transition: all var(--transition);
	}

	.primary-btn:hover:not(:disabled) {
		background: var(--accent-hover);
	}

	.primary-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	/* --- Provider styles --- */

	.provider-card {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 12px;
		margin-top: 8px;
	}

	.provider-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.provider-info {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.external-dot-settings {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #f59e0b;
		flex-shrink: 0;
	}

	.provider-name {
		font-weight: 500;
		font-size: 0.875rem;
		color: var(--text-primary);
	}

	.provider-url {
		display: block;
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	.provider-actions {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.small-toggle {
		transform: scale(0.75);
		margin: 0;
	}

	.provider-meta {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-top: 8px;
		font-size: 0.8rem;
		color: var(--text-muted);
	}

	.provider-models {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--border-light);
	}

	.model-checklist {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.model-check-item {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 8px;
		border-radius: 4px;
		font-size: 0.85rem;
		border: 1px solid var(--border);
	}

	.model-check-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.model-check-name {
		color: var(--text-primary);
		font-weight: 500;
	}

	.model-check-size {
		color: var(--text-muted);
		font-size: 0.75rem;
	}

	.model-role-row {
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
	}

	.role-chip {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 8px;
		border-radius: 12px;
		font-size: 0.75rem;
		cursor: pointer;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--text-secondary);
		transition: all 0.15s;
		user-select: none;
	}

	.role-chip input[type='checkbox'] {
		display: none;
	}

	.role-chip.active {
		background: var(--accent-light);
		color: var(--accent);
		border-color: var(--accent);
		font-weight: 600;
	}

	.link-btn {
		background: none;
		border: none;
		color: var(--accent);
		cursor: pointer;
		padding: 0;
		font-size: inherit;
		font-family: inherit;
		text-decoration: underline;
	}

	.link-btn:hover {
		opacity: 0.8;
	}

	.provider-form {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 16px;
		margin-top: 12px;
		background: var(--bg-secondary);
	}

	.form-title {
		font-size: 0.9rem;
		font-weight: 600;
		margin-bottom: 12px;
		color: var(--text-primary);
	}

	.text-input {
		width: 100%;
		padding: 8px 12px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--bg-primary);
		color: var(--text-primary);
		font-size: 0.85rem;
		outline: none;
		transition: border-color var(--transition);
	}

	.text-input:focus {
		border-color: var(--accent);
	}

	.warning-text {
		color: #f59e0b;
		font-size: 0.75rem;
		margin-top: 4px;
	}

	.success-text {
		color: var(--accent);
		font-size: 0.85rem;
		margin-bottom: 8px;
	}
</style>
