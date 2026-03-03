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
		margin-left: 8px;
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
		transition: background var(--transition);
		appearance: none;
		-webkit-appearance: none;
		background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%23555555' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E");
		background-repeat: no-repeat;
		background-position: right 8px center;
	}

	select:hover {
		background-color: var(--bg-hover);
	}

	select:focus {
		background-color: var(--bg-hover);
	}
</style>
