<script lang="ts">
	import { getModels, getActiveModel, switchModel } from '$lib/stores.svelte';
</script>

<div class="model-selector">
	<select
		value={getActiveModel()?.id ?? ''}
		onchange={(e) => switchModel((e.target as HTMLSelectElement).value)}
	>
		{#each getModels() as model}
			<option value={model.id}>
				{model.name}
			</option>
		{/each}
	</select>
</div>

<style>
	.model-selector {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-left: 8px;
	}

	select {
		padding: 6px 12px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		background: var(--bg-input);
		color: var(--text-primary);
		outline: none;
		font-size: 0.85rem;
		cursor: pointer;
		transition: border-color var(--transition);
	}

	select:hover {
		border-color: var(--text-muted);
	}

	select:focus {
		border-color: var(--accent);
	}
</style>
