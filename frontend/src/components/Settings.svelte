<script lang="ts">
	import {
		getSettingsOpen,
		toggleSettings,
		applyTheme,
		syncConfig,
		getModels,
		loadModels,
		loadStatus,
		unloadModel,
		deleteAllConversations,
		showNotification,
		getConversations,
		getEngineStatus
	} from '$lib/stores.svelte';
	import * as api from '$lib/api';
	import type { AppConfig, ModelInfo } from '$lib/types';

	let config = $state<AppConfig | null>(null);
	let loading = $state(false);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let defaultPrompt = $state('');
	let editingModelId = $state<string | null>(null);
	let editingName = $state('');
	let deleteModelConfirm = $state<string | null>(null);
	let deleteAllStep = $state(0);
	let deleteAllTimeout: ReturnType<typeof setTimeout> | null = null;
	let systemPromptOpen = $state(false);
	let unloadingModelId = $state<string | null>(null);
	let dragModelId = $state<string | null>(null);
	let dragOverModelId = $state<string | null>(null);

	// Loaded models derived from model list
	let loadedModels = $derived(getModels().filter((m) => m.loaded));
	let status = $derived(getEngineStatus());

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
			loadConfig();
		} else {
			config = null;
			error = null;
			editingModelId = null;
			deleteModelConfirm = null;
			deleteAllStep = 0;
			systemPromptOpen = false;
		}
	});

	async function loadConfig() {
		loading = true;
		error = null;
		try {
			config = await api.getConfig();
			await Promise.all([loadModels(), loadStatus()]);
			try {
				defaultPrompt = await api.getDefaultSystemPrompt();
			} catch {
				defaultPrompt = '';
			}
		} catch (err) {
			console.error('Failed to load settings:', err);
			error = 'Failed to load settings. Please try again.';
		} finally {
			loading = false;
		}
	}

	async function save() {
		if (!config || saving) return;
		saving = true;
		error = null;
		try {
			config = await api.updateConfig(config);
			showNotification('Settings saved.', 'info');
		} catch (err) {
			console.error('Failed to save settings:', err);
			error = 'Failed to save settings. Please try again.';
		} finally {
			saving = false;
		}
	}

	function setTheme(theme: 'light' | 'dark' | 'system') {
		if (!config) return;
		config.theme = theme;
		applyTheme(theme);
		api.updateConfig(config).then((c) => {
			config = c;
			syncConfig(c);
		});
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

	function tierLabel(tier: string): string {
		switch (tier) {
			case 'lite': return 'Lite';
			case 'standard': return 'Standard';
			case 'pro': return 'Pro';
			default: return 'Custom';
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
		if (!config) return;
		config.system_prompt = '';
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
					<!-- ==================== GENERAL ==================== -->
					<section class="section">
						<h4 class="section-title">General</h4>

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

					<!-- ==================== LOADED MODELS ==================== -->
					{#if loadedModels.length > 0}
						<section class="section">
							<h4 class="section-title">Loaded Models</h4>
							<div class="model-list">
								{#each loadedModels as model (model.id)}
									<div class="model-item loaded-item">
										<div class="model-info">
											<div class="model-name-row">
												<span class="loaded-dot-settings"></span>
												<span class="model-name">{model.name}</span>
												{#if model.active}
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
											<button
												class="small-btn unload-btn"
												onclick={() => handleUnload(model.id)}
												disabled={unloadingModelId === model.id}
											>
												{unloadingModelId === model.id ? 'Unloading...' : 'Unload'}
											</button>
										</div>
									</div>
								{/each}
							</div>
						</section>
					{/if}

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
													<span class="badge tier-badge tier-{model.tier}">{tierLabel(model.tier)}</span>
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
														<span class="badge tier-badge tier-{model.tier}">{tierLabel(model.tier)}</span>
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

						<p class="field-desc" style="margin-top: 12px;">
							To add a new model, place a .gguf file in the models folder.
						</p>

						<div class="button-row">
							<button class="secondary-btn" onclick={() => api.openFolder('models')}>
								Open Models Folder
							</button>
							<button class="secondary-btn" onclick={() => api.refreshModels().then(() => loadModels())}>
								Refresh
							</button>
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

						{#if config.pro_mode}
							<div class="field">
								<button class="collapsible" onclick={() => (systemPromptOpen = !systemPromptOpen)}>
									<span>
										System Prompt (Advanced)
										{#if config.system_prompt}
											<span class="modified-badge">Modified</span>
										{/if}
									</span>
									<svg
										width="14" height="14" viewBox="0 0 24 24"
										fill="none" stroke="currentColor" stroke-width="2"
										class:rotated={systemPromptOpen}
									>
										<polyline points="6 9 12 15 18 9" />
									</svg>
								</button>
								{#if systemPromptOpen}
									<div class="collapsible-content">
										<textarea
											class="textarea system-prompt-textarea"
											rows="6"
											bind:value={config.system_prompt}
											placeholder={defaultPrompt}
										></textarea>
										<div class="button-row" style="margin-top: 8px;">
											<button class="small-btn muted" onclick={resetSystemPrompt}>
												Reset to Default
											</button>
										</div>
									</div>
								{/if}
							</div>
						{/if}
					</section>

					<!-- ==================== INFERENCE PARAMETERS (Pro) ==================== -->
					{#if config.pro_mode}
						<section class="section">
							<h4 class="section-title">Inference Parameters</h4>

							<div class="field">
								<div class="slider-header">
									<span class="field-label">Temperature</span>
									<span class="slider-value">{config.temperature.toFixed(2)}</span>
								</div>
								<p class="field-desc">Higher = more creative, lower = more focused</p>
								<input type="range" class="slider" min="0" max="2" step="0.05" bind:value={config.temperature} />
							</div>

							<div class="field">
								<div class="slider-header">
									<span class="field-label">Top P</span>
									<span class="slider-value">{config.top_p.toFixed(2)}</span>
								</div>
								<p class="field-desc">Controls diversity of word choices</p>
								<input type="range" class="slider" min="0" max="1" step="0.05" bind:value={config.top_p} />
							</div>

							<div class="field">
								<span class="field-label">Top K</span>
								<p class="field-desc">Limits the number of word choices considered</p>
								<input type="number" class="number-input" min="0" max="200" bind:value={config.top_k} />
							</div>

							<div class="field">
								<div class="slider-header">
									<span class="field-label">Repeat Penalty</span>
									<span class="slider-value">{config.repeat_penalty.toFixed(2)}</span>
								</div>
								<p class="field-desc">Reduces repetition in responses</p>
								<input type="range" class="slider" min="0" max="2" step="0.05" bind:value={config.repeat_penalty} />
							</div>

							<div class="field">
								<span class="field-label">Max Tokens</span>
								<p class="field-desc">Maximum response length. 0 = no limit.</p>
								<input type="number" class="number-input" min="0" max="32768" bind:value={config.max_tokens} />
							</div>

							<div class="field">
								<span class="field-label">Seed</span>
								<p class="field-desc">Fixed seed for reproducible responses. -1 = random.</p>
								<input type="number" class="number-input" min="-1" max="999999" bind:value={config.seed} />
							</div>

							<div class="button-row">
								<button class="secondary-btn" onclick={resetInferenceDefaults}>
									Reset to Defaults
								</button>
							</div>
						</section>
					{/if}

					<!-- ==================== SERVER CONFIG (Pro) ==================== -->
					{#if config.pro_mode}
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
					{/if}

					<!-- ==================== SAVE BUTTON ==================== -->
					{#if error}
						<p class="error-msg">{error}</p>
					{/if}
					<button class="save-btn" onclick={save} disabled={saving}>
						{saving ? 'Saving...' : 'Save Settings'}
					</button>

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

	/* Toggle */
	.toggle-row {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
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

	.tier-badge {
		background: var(--bg-tertiary);
		color: var(--text-muted);
	}

	.tier-lite {
		background: #dbeafe;
		color: #1e40af;
	}

	.tier-standard {
		background: #d1fae5;
		color: #065f46;
	}

	.tier-pro {
		background: #ede9fe;
		color: #5b21b6;
	}

	:global([data-theme='dark']) .tier-lite {
		background: #1e3a5f;
		color: #93c5fd;
	}

	:global([data-theme='dark']) .tier-standard {
		background: #064e3b;
		color: #6ee7b7;
	}

	:global([data-theme='dark']) .tier-pro {
		background: #3b0764;
		color: #c4b5fd;
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

	/* Collapsible */
	.collapsible {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 8px 0;
		color: var(--text-secondary);
		font-size: 0.85rem;
		font-weight: 500;
	}

	.collapsible:hover {
		color: var(--text-primary);
	}

	.collapsible svg {
		transition: transform var(--transition);
	}

	.collapsible svg.rotated {
		transform: rotate(180deg);
	}

	.collapsible-content {
		padding-top: 8px;
	}

	.modified-badge {
		font-size: 0.7rem;
		padding: 1px 6px;
		border-radius: 4px;
		background: var(--accent-light);
		color: var(--accent);
		margin-left: 6px;
		font-weight: 600;
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
</style>
