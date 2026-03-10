<script lang="ts">
	import {
		getUnloadPopup,
		dismissUnloadPopup,
		confirmUnloadAndLoad,
		getModels
	} from '$lib/stores.svelte';

	let popup = $derived(getUnloadPopup());
	let models = $derived(getModels());
	let selectedIds = $state<Set<string>>(new Set());
	let confirming = $state(false);

	// Active (loaded) models, excluding the target model
	let activeModels = $derived(
		models.filter((m) => m.loaded && m.id !== popup?.targetModelId)
	);

	// Target model info
	let targetModel = $derived(
		popup ? models.find((m) => m.id === popup.targetModelId) : null
	);

	// Reset selection when popup appears
	$effect(() => {
		if (popup) {
			selectedIds = new Set();
			confirming = false;
		}
	});

	function formatGB(bytes: number): string {
		return (bytes / (1024 * 1024 * 1024)).toFixed(1);
	}

	function formatSize(bytes?: number): string {
		if (!bytes) return '';
		const gb = bytes / (1024 * 1024 * 1024);
		if (gb >= 1) return gb.toFixed(1) + ' GB';
		const mb = bytes / (1024 * 1024);
		return mb.toFixed(0) + ' MB';
	}

	function toggleModel(id: string) {
		const next = new Set(selectedIds);
		if (next.has(id)) {
			next.delete(id);
		} else {
			next.add(id);
		}
		selectedIds = next;
	}

	async function handleConfirm() {
		if (selectedIds.size === 0) return;
		confirming = true;
		await confirmUnloadAndLoad([...selectedIds]);
		confirming = false;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && popup) {
			dismissUnloadPopup();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if popup}
	<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
	<div class="overlay" onclick={dismissUnloadPopup}>
		<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
		<div class="popup" onclick={(e) => e.stopPropagation()}>
			<h3 class="popup-title">Not Enough Memory</h3>

			<p class="popup-desc">
				{#if targetModel}
					<strong>{targetModel.name}</strong> needs ~{formatGB(popup.memoryError.required_bytes)} GB
				{:else}
					This model needs ~{formatGB(popup.memoryError.required_bytes)} GB
				{/if}
				but only ~{formatGB(popup.memoryError.available_bytes)} GB is available.
			</p>

			<p class="popup-desc">Select models to unload:</p>

			{#if activeModels.length === 0}
				<p class="popup-desc muted">No other models are currently loaded.</p>
			{:else}
				<div class="model-list">
					{#each activeModels as model (model.id)}
						<label class="model-row" class:selected={selectedIds.has(model.id)}>
							<input
								type="checkbox"
								checked={selectedIds.has(model.id)}
								onchange={() => toggleModel(model.id)}
							/>
							<div class="model-details">
								<span class="model-name">{model.name}</span>
								{#if model.size}
									<span class="model-size">{formatSize(model.size)}</span>
								{/if}
							</div>
						</label>
					{/each}
				</div>
			{/if}

			<div class="popup-actions">
				<button class="cancel-btn" onclick={dismissUnloadPopup}>Cancel</button>
				<button
					class="confirm-btn"
					onclick={handleConfirm}
					disabled={selectedIds.size === 0 || confirming}
				>
					{confirming ? 'Loading...' : 'Unload & Load'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.4);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 300;
		animation: fade-in 0.15s ease;
	}

	@keyframes fade-in {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	.popup {
		background: var(--bg-primary);
		border: 1px solid var(--border);
		border-radius: 12px;
		box-shadow: var(--shadow-lg);
		padding: 24px;
		width: 380px;
		max-width: 90vw;
		max-height: 80vh;
		overflow-y: auto;
		animation: popup-in 0.2s ease;
	}

	@keyframes popup-in {
		from { opacity: 0; transform: scale(0.95); }
		to { opacity: 1; transform: scale(1); }
	}

	.popup-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 12px;
	}

	.popup-desc {
		font-size: 0.85rem;
		color: var(--text-secondary);
		margin-bottom: 12px;
		line-height: 1.4;
	}

	.popup-desc.muted {
		color: var(--text-muted);
		font-style: italic;
	}

	.model-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin-bottom: 16px;
	}

	.model-row {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 10px 12px;
		border-radius: var(--radius);
		cursor: pointer;
		transition: background var(--transition);
	}

	.model-row:hover {
		background: var(--bg-hover);
	}

	.model-row.selected {
		background: var(--accent-light);
	}

	.model-row input[type="checkbox"] {
		accent-color: var(--accent);
		width: 16px;
		height: 16px;
		flex-shrink: 0;
	}

	.model-details {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.model-name {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
	}

	.model-size {
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	.popup-actions {
		display: flex;
		gap: 8px;
		justify-content: flex-end;
	}

	.cancel-btn {
		padding: 8px 16px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		font-size: 0.85rem;
		color: var(--text-secondary);
		background: var(--bg-primary);
		cursor: pointer;
		transition: all var(--transition);
	}

	.cancel-btn:hover {
		border-color: var(--text-muted);
		color: var(--text-primary);
	}

	.confirm-btn {
		padding: 8px 16px;
		border-radius: var(--radius);
		background: var(--accent);
		color: white;
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
		transition: background var(--transition);
	}

	.confirm-btn:hover:not(:disabled) {
		background: var(--accent-hover);
	}

	.confirm-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
