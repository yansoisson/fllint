<script lang="ts">
	import '../app.css';
	import Sidebar from '$components/Sidebar.svelte';
	import ModelSelector from '$components/ModelSelector.svelte';
	import Settings from '$components/Settings.svelte';
	import Toast from '$components/Toast.svelte';
	import UnloadPopup from '$components/UnloadPopup.svelte';
	import { goto } from '$app/navigation';
	import {
		initApp,
		getInitError,
		toggleSidebar,
		getIsStreaming
	} from '$lib/stores.svelte';

	let { children } = $props();

	$effect(() => {
		initApp();
	});
</script>

<div class="app">
	<Sidebar />
	<main>
		<header>
			<button class="icon-btn" onclick={toggleSidebar} title="Toggle sidebar">
				<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<line x1="3" y1="6" x2="21" y2="6" />
					<line x1="3" y1="12" x2="21" y2="12" />
					<line x1="3" y1="18" x2="21" y2="18" />
				</svg>
			</button>
			<ModelSelector />
			<div class="spacer"></div>
			<button class="icon-btn" onclick={() => goto('/settings')} title="Settings">
				<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="12" cy="12" r="3" />
					<path
						d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
					/>
				</svg>
			</button>
		</header>
		{#if getInitError()}
			<div class="init-error">
				<span>{getInitError()}</span>
				<button class="retry-btn" onclick={() => initApp()}>Retry</button>
			</div>
		{/if}
		{@render children()}
	</main>
</div>

<Settings />
<UnloadPopup />
<Toast />

<style>
	.app {
		display: flex;
		height: 100vh;
		overflow: hidden;
	}

	main {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	header {
		display: flex;
		align-items: center;
		padding: 0 16px;
		background: var(--bg-primary);
		height: var(--header-height);
		flex-shrink: 0;
	}

	.spacer {
		flex: 1;
	}

	.icon-btn {
		width: 36px;
		height: 36px;
		border-radius: var(--radius);
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-secondary);
		transition: all var(--transition);
	}

	.icon-btn:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.init-error {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 12px;
		padding: 10px 16px;
		background: var(--error-bg, #fef2f2);
		border-bottom: 1px solid var(--error-border, #fecaca);
		color: var(--error-text, #991b1b);
		font-size: 0.875rem;
	}

	.retry-btn {
		padding: 4px 12px;
		border-radius: var(--radius);
		background: var(--error-text, #991b1b);
		color: white;
		font-size: 0.8rem;
		cursor: pointer;
		white-space: nowrap;
	}

	.retry-btn:hover {
		opacity: 0.9;
	}
</style>
