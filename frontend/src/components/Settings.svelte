<script lang="ts">
	import { getSettingsOpen, toggleSettings } from '$lib/stores.svelte';
	import { getConfig, updateConfig } from '$lib/api';
	import type { AppConfig } from '$lib/types';

	let config = $state<AppConfig | null>(null);
	let loading = $state(false);

	$effect(() => {
		if (getSettingsOpen() && !config) {
			loadConfig();
		}
	});

	async function loadConfig() {
		loading = true;
		try {
			config = await getConfig();
		} catch (err) {
			console.error('Failed to load config:', err);
		} finally {
			loading = false;
		}
	}

	async function save() {
		if (!config) return;
		try {
			config = await updateConfig(config);
			toggleSettings();
		} catch (err) {
			console.error('Failed to save config:', err);
		}
	}
</script>

{#if getSettingsOpen()}
	<div class="overlay" onclick={toggleSettings} role="presentation"></div>
	<div class="panel">
		<div class="panel-header">
			<h3>Settings</h3>
			<button class="close-btn" onclick={toggleSettings}>&times;</button>
		</div>

		{#if loading}
			<p class="loading">Loading...</p>
		{:else if config}
			<div class="form">
				<label>
					<span>Data Directory</span>
					<input bind:value={config.data_dir} />
				</label>
				<label>
					<span>Models Directory</span>
					<input bind:value={config.models_dir} />
				</label>
				<label>
					<span>Port</span>
					<input type="number" bind:value={config.port} />
				</label>
				<button class="save-btn" onclick={save}>Save</button>
			</div>
		{/if}
	</div>
{/if}

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		z-index: 10;
	}

	.panel {
		position: fixed;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 24px;
		z-index: 11;
		min-width: 380px;
		max-width: 90vw;
	}

	.panel-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
	}

	h3 {
		font-size: 1.1rem;
		font-weight: 600;
	}

	.close-btn {
		font-size: 20px;
		color: var(--text-muted);
		padding: 4px 8px;
		border-radius: 4px;
	}

	.close-btn:hover {
		color: var(--text-primary);
		background: var(--bg-tertiary);
	}

	.form {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	label {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	label span {
		font-size: 0.85rem;
		color: var(--text-secondary);
		font-weight: 500;
	}

	input {
		padding: 10px 14px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		background: var(--bg-input);
		outline: none;
		transition: border-color var(--transition);
	}

	input:focus {
		border-color: var(--accent);
	}

	.save-btn {
		padding: 10px 20px;
		border-radius: var(--radius);
		background: var(--accent);
		color: white;
		font-weight: 600;
		margin-top: 4px;
		transition: background var(--transition);
	}

	.save-btn:hover {
		background: var(--accent-hover);
	}

	.loading {
		color: var(--text-muted);
		text-align: center;
		padding: 20px;
	}
</style>
