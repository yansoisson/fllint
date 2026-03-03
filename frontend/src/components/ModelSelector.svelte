<script lang="ts">
	import { getModels, getActiveModel, switchModel, getEngineStatus } from '$lib/stores.svelte';

	let status = $derived(getEngineStatus());
	let isStarting = $derived(status?.engine_state === 'starting');
	let models = $derived(getModels());
	let activeModel = $derived(getActiveModel());
</script>

<div class="model-selector">
	<select
		value={activeModel?.id ?? ''}
		onchange={(e) => switchModel((e.target as HTMLSelectElement).value)}
		disabled={isStarting}
	>
		{#if models.length === 0}
			<option value="" disabled>No models found</option>
		{:else if !activeModel}
			<option value="" disabled>Select a model...</option>
		{/if}
		{#each models as model}
			<option value={model.id}>{model.name}</option>
		{/each}
	</select>
	{#if isStarting}
		<div class="loading-dot"></div>
	{/if}
</div>

<style>
	.model-selector {
		display: flex;
		align-items: center;
		margin-left: 8px;
		gap: 8px;
	}

	select {
		padding: 6px 28px 6px 12px;
		border-radius: var(--radius);
		border: none;
		background: transparent;
		color: var(--text-primary);
		outline: none;
		font-size: 0.9375rem;
		font-weight: 600;
		cursor: pointer;
		transition: background var(--transition), opacity var(--transition);
		appearance: none;
		-webkit-appearance: none;
		background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%23555555' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E");
		background-repeat: no-repeat;
		background-position: right 8px center;
	}

	select:disabled {
		opacity: 0.5;
		cursor: default;
	}

	select:hover:not(:disabled) {
		background-color: var(--bg-hover);
	}

	select:focus {
		background-color: var(--bg-hover);
	}

	.loading-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--accent);
		animation: pulse 1s ease-in-out infinite;
		flex-shrink: 0;
	}

	@keyframes pulse {
		0%, 100% { opacity: 0.3; }
		50% { opacity: 1; }
	}
</style>
