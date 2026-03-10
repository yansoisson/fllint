<script lang="ts">
	import {
		getModels,
		getEffectiveModelId,
		selectModelForTab,
		getEngineStatus,
		getPinnedModelIds
	} from '$lib/stores.svelte';
	import type { ModelInfo, EngineStatusInfo } from '$lib/types';

	let open = $state(false);
	let otherExpanded = $state(false);
	let popoverEl = $state<HTMLDivElement | null>(null);
	let triggerEl = $state<HTMLButtonElement | null>(null);

	let models = $derived(getModels());
	let effectiveModelId = $derived(getEffectiveModelId());
	let status = $derived(getEngineStatus());

	// Build a lookup for engine status by model_id
	let engineMap = $derived.by(() => {
		const map: Record<string, EngineStatusInfo> = {};
		if (status?.engines) {
			for (const e of status.engines) {
				map[e.model_id] = e;
			}
		}
		return map;
	});

	// Pinned model IDs: from config if set, otherwise default to tier-based
	let configPinnedIds = $derived(getPinnedModelIds());
	let effectivePinnedIds = $derived.by(() => {
		if (configPinnedIds.length > 0) return configPinnedIds;
		return models
			.filter((m) => m.tier === 'lite' || m.tier === 'standard' || m.tier === 'pro')
			.map((m) => m.id);
	});

	// Pinned models in config order, other models are everything else
	let pinnedModels = $derived.by(() => {
		const result: ModelInfo[] = [];
		for (const id of effectivePinnedIds) {
			const m = models.find((model) => model.id === id);
			if (m) result.push(m);
		}
		return result;
	});
	let otherModels = $derived(models.filter((m) => !effectivePinnedIds.includes(m.id)));

	let currentModel = $derived(models.find((m) => m.id === effectiveModelId) ?? null);

	function formatSize(bytes?: number): string {
		if (!bytes) return '';
		const gb = bytes / (1024 * 1024 * 1024);
		if (gb >= 1) return gb.toFixed(1) + ' GB';
		const mb = bytes / (1024 * 1024);
		return mb.toFixed(0) + ' MB';
	}

	function isModelStarting(modelId: string): boolean {
		return engineMap[modelId]?.engine_state === 'starting';
	}

	function getModelProgress(modelId: string): number {
		return engineMap[modelId]?.load_progress ?? 0;
	}

	function selectModel(modelId: string) {
		selectModelForTab(modelId);
		open = false;
	}

	function toggle() {
		open = !open;
		if (!open) otherExpanded = false;
	}

	function handleClickOutside(e: MouseEvent) {
		if (!open) return;
		const target = e.target as Node;
		if (popoverEl?.contains(target) || triggerEl?.contains(target)) return;
		open = false;
		otherExpanded = false;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) {
			open = false;
			otherExpanded = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} onkeydown={handleKeydown} />

<div class="model-selector-wrapper">
	<button class="selector-trigger" bind:this={triggerEl} onclick={toggle}>
		{#if currentModel}
			{#if currentModel.loaded}
				<span class="loaded-dot"></span>
			{/if}
			<span class="trigger-label">{currentModel.name}</span>
		{:else}
			<span class="trigger-label placeholder">Select model</span>
		{/if}
		<svg class="chevron" class:open width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
			<polyline points="6 9 12 15 18 9" />
		</svg>
	</button>

	{#if open}
		<div class="popover" bind:this={popoverEl}>
			{#if pinnedModels.length === 0 && otherModels.length === 0}
				<div class="empty-state">No models found</div>
			{/if}

			{#each pinnedModels as model (model.id)}
				{@const starting = isModelStarting(model.id)}
				{@const progress = getModelProgress(model.id)}
				{@const isSelected = model.id === effectiveModelId}
				<button
					class="model-option"
					class:selected={isSelected}
					onclick={() => selectModel(model.id)}
				>
					<div class="option-left">
						<span class="status-indicator">
							{#if starting}
								<span class="loading-spinner"></span>
							{:else if model.loaded}
								<span class="loaded-dot"></span>
							{/if}
						</span>
						<div class="option-text">
							<span class="option-name">{model.name}</span>
							{#if model.size}
								<span class="option-size">{formatSize(model.size)}</span>
							{/if}
						</div>
					</div>
					{#if isSelected}
						<svg class="check-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
							<polyline points="20 6 9 17 4 12" />
						</svg>
					{/if}
					{#if starting && progress > 0}
						<div class="option-progress">
							<div class="option-progress-bar" style="width: {Math.round(progress * 100)}%"></div>
						</div>
					{/if}
				</button>
			{/each}

			{#if otherModels.length > 0}
				<div class="separator"></div>
				<button class="other-toggle" onclick={() => (otherExpanded = !otherExpanded)}>
					<span>Other models</span>
					<svg class="chevron-small" class:expanded={otherExpanded} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<polyline points="9 6 15 12 9 18" />
					</svg>
				</button>
				{#if otherExpanded}
					{#each otherModels as model (model.id)}
						{@const starting = isModelStarting(model.id)}
						{@const progress = getModelProgress(model.id)}
						{@const isSelected = model.id === effectiveModelId}
						<button
							class="model-option other"
							class:selected={isSelected}
							onclick={() => selectModel(model.id)}
						>
							<div class="option-left">
								<span class="status-indicator">
									{#if starting}
										<span class="loading-spinner"></span>
									{:else if model.loaded}
										<span class="loaded-dot"></span>
									{/if}
								</span>
								<div class="option-text">
									<span class="option-name">{model.name}</span>
									{#if model.size}
										<span class="option-size">{formatSize(model.size)}</span>
									{/if}
								</div>
							</div>
							{#if isSelected}
								<svg class="check-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
									<polyline points="20 6 9 17 4 12" />
								</svg>
							{/if}
							{#if starting && progress > 0}
								<div class="option-progress">
									<div class="option-progress-bar" style="width: {Math.round(progress * 100)}%"></div>
								</div>
							{/if}
						</button>
					{/each}
				{/if}
			{/if}
		</div>
	{/if}
</div>

<style>
	.model-selector-wrapper {
		position: relative;
		margin-left: 8px;
	}

	.selector-trigger {
		display: flex;
		align-items: center;
		gap: 4px;
		padding: 6px 10px;
		border-radius: var(--radius);
		background: transparent;
		cursor: pointer;
		transition: background var(--transition);
	}

	.selector-trigger:hover {
		background: var(--bg-hover);
	}

	.trigger-label {
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--text-primary);
	}

	.trigger-label.placeholder {
		color: var(--text-muted);
	}

	.chevron {
		color: var(--text-muted);
		transition: transform var(--transition);
		flex-shrink: 0;
	}

	.chevron.open {
		transform: rotate(180deg);
	}

	/* Popover */
	.popover {
		position: absolute;
		top: calc(100% + 6px);
		left: 0;
		min-width: 220px;
		background: var(--bg-primary);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		box-shadow: var(--shadow-lg);
		z-index: 200;
		padding: 4px;
		animation: popover-in 0.15s ease;
	}

	@keyframes popover-in {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.empty-state {
		padding: 16px;
		text-align: center;
		color: var(--text-muted);
		font-size: 0.85rem;
	}

	/* Model option */
	.model-option {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 10px 12px;
		border-radius: 6px;
		cursor: pointer;
		transition: background var(--transition);
		position: relative;
		overflow: hidden;
	}

	.model-option:hover {
		background: var(--bg-hover);
	}

	.model-option.selected {
		background: var(--accent-light);
	}

	.model-option.other {
		padding-left: 24px;
	}

	.option-left {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
	}

	.status-indicator {
		width: 10px;
		height: 10px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.loaded-dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--accent);
		flex-shrink: 0;
	}

	.loading-spinner {
		width: 10px;
		height: 10px;
		border: 1.5px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.option-text {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.option-name {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.option-size {
		font-size: 0.75rem;
		color: var(--text-muted);
		line-height: 1.2;
	}

	.check-icon {
		color: var(--accent);
		flex-shrink: 0;
		margin-left: 8px;
	}

	/* Progress bar overlay at bottom of option */
	.option-progress {
		position: absolute;
		bottom: 0;
		left: 0;
		right: 0;
		height: 2px;
		background: var(--bg-tertiary);
	}

	.option-progress-bar {
		height: 100%;
		background: var(--accent);
		transition: width 0.3s ease;
	}

	/* Separator */
	.separator {
		height: 1px;
		background: var(--border-light);
		margin: 4px 8px;
	}

	/* Other models toggle */
	.other-toggle {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 8px 12px;
		border-radius: 6px;
		cursor: pointer;
		font-size: 0.8rem;
		color: var(--text-muted);
		font-weight: 500;
		transition: all var(--transition);
	}

	.other-toggle:hover {
		background: var(--bg-hover);
		color: var(--text-secondary);
	}

	.chevron-small {
		transition: transform var(--transition);
		flex-shrink: 0;
	}

	.chevron-small.expanded {
		transform: rotate(90deg);
	}
</style>
