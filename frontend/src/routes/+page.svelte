<script lang="ts">
	import Sidebar from '$components/Sidebar.svelte';
	import ChatWindow from '$components/ChatWindow.svelte';
	import InputBar from '$components/InputBar.svelte';
	import ModelSelector from '$components/ModelSelector.svelte';
	import Settings from '$components/Settings.svelte';
	import { loadConversations, loadModels, toggleSidebar, toggleSettings } from '$lib/stores.svelte';

	$effect(() => {
		loadConversations();
		loadModels();
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
			<button class="icon-btn" onclick={toggleSettings} title="Settings">
				<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="12" cy="12" r="3" />
					<path
						d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
					/>
				</svg>
			</button>
		</header>
		<ChatWindow />
		<InputBar />
	</main>
</div>

<Settings />

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
		padding: 0 12px;
		border-bottom: 1px solid var(--border);
		background: var(--bg-secondary);
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
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}
</style>
