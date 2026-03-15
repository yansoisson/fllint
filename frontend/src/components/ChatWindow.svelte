<script lang="ts">
	import MessageBubble from './MessageBubble.svelte';
	import {
		getMessages,
		getIsStreaming,
		getStreamingContent,
		getStreamingReasoning,
		getThinkingDuration,
		getEngineStatus,
		getChatError,
		clearChatError,
		refreshModels,
		openSettingsToTab,
		isDownloadActive,
		getActiveDownloadsState,
		startDownloadPolling,
		startModelDownload
	} from '$lib/stores.svelte';

	let chatContainer: HTMLDivElement;
	let hasMessages = $derived(getMessages().length > 0 || getIsStreaming());

	// Start download polling when we detect an active download (e.g. auto-download on first launch)
	$effect(() => {
		if (isDownloadActive()) {
			startDownloadPolling();
		}
	});

	$effect(() => {
		const _ = getMessages();
		const __ = getStreamingContent();
		const ___ = getStreamingReasoning();
		if (chatContainer) {
			requestAnimationFrame(() => {
				chatContainer.scrollTop = chatContainer.scrollHeight;
			});
		}
	});
</script>

<div class="chat-window" class:has-messages={hasMessages} bind:this={chatContainer}>
	{#each getMessages() as message}
		<MessageBubble {message} />
	{/each}

	{#if getIsStreaming() && (getStreamingContent() || getStreamingReasoning())}
		<MessageBubble message={{
			role: 'assistant',
			content: getStreamingContent(),
			reasoning: getStreamingReasoning() || undefined,
			thinking_duration: getThinkingDuration() ?? undefined
		}} isStreamingMessage={true} />
	{/if}

	{#if getChatError()}
		<div class="error-banner">
			<span>{getChatError()}</span>
			<button class="dismiss-btn" onclick={clearChatError}>Dismiss</button>
		</div>
	{/if}

	{#if getMessages().length === 0 && !getIsStreaming()}
		<div class="empty">
			{#if !getEngineStatus()?.has_binary && !getEngineStatus()?.has_models}
				<h2>Welcome to Fllint</h2>
				<p>To get started, you need two things:</p>
				<div class="setup-steps">
					<div class="step">
						<strong>1. llama-server</strong>
						<span>Download from llama.cpp releases and place in <code>bin/</code></span>
					</div>
					<div class="step">
						<strong>2. A model file</strong>
						<span>Download a .gguf model and place in <code>models/</code></span>
					</div>
				</div>
				<button class="refresh-btn" onclick={refreshModels}>
					I've placed the files — refresh
				</button>
			{:else if !getEngineStatus()?.has_binary}
				<h2>Almost there</h2>
				<p>
					Model files found, but llama-server is missing.
					Place it in the <code>bin/</code> folder.
				</p>
				<button class="refresh-btn" onclick={refreshModels}>Refresh</button>
			{:else if !getEngineStatus()?.has_models && isDownloadActive()}
				{@const dl = getActiveDownloadsState().find((d) => d.state === 'downloading' || d.state === 'queued')}
				<h2>Downloading model...</h2>
				<p>{dl?.display_name ?? 'Model'} — this may take a few minutes.</p>
				<div class="spinner"></div>
				{#if dl && dl.total_bytes > 0}
					<div class="download-progress">
						<div class="download-bar">
							<div class="download-fill" style="width: {Math.round((dl.done_bytes / dl.total_bytes) * 100)}%"></div>
						</div>
						<span class="download-pct">{Math.round((dl.done_bytes / dl.total_bytes) * 100)}%</span>
					</div>
				{/if}
			{:else if !getEngineStatus()?.has_models}
				<h2>Welcome to Fllint</h2>
				<p>Download a local model to get started, or connect an external provider.</p>
				<div class="welcome-actions">
					<button class="refresh-btn" onclick={() => { startModelDownload('lite-qwen3.5-2b'); startDownloadPolling(); }}>
						Download Lite Model
					</button>
					<button class="refresh-btn secondary" onclick={() => openSettingsToTab('models')}>
						Browse All Models
					</button>
				</div>
				<p class="manual-hint">
					Or place a .gguf file in the <code>models/</code> folder manually.
					<button class="link-btn" onclick={refreshModels}>Refresh</button>
				</p>
			{:else if getEngineStatus()?.engine_state === 'starting'}
				<h2>Loading model...</h2>
				<p>This can take a minute for larger models.</p>
				<div class="spinner"></div>
			{:else if getEngineStatus()?.engine_state === 'error'}
				<h2>Something went wrong</h2>
				<p class="error-text">{getEngineStatus()?.error}</p>
			{:else if getEngineStatus()?.engine_state === 'idle'}
				<h2>Welcome to Fllint</h2>
				<p>Select a model from the dropdown above to get started.</p>
			{:else}
				<h2>What's on your mind?</h2>
			{/if}
		</div>
	{/if}
</div>

<style>
	.chat-window {
		overflow-y: auto;
		padding: 24px max(24px, calc((100% - var(--content-max-width)) / 2));
		display: flex;
		flex-direction: column;
	}

	.chat-window.has-messages {
		flex: 1;
	}

	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		color: var(--text-muted);
		text-align: center;
		gap: 8px;
		padding: 32px 0;
		max-width: var(--content-max-width);
		width: 100%;
		align-self: center;
	}

	.empty h2 {
		margin-bottom: 4px;
		font-weight: 500;
		color: var(--text-primary);
		font-size: 1.8rem;
	}

	.empty p {
		color: var(--text-secondary);
	}

	.empty code {
		background: var(--bg-tertiary);
		padding: 2px 6px;
		border-radius: 4px;
		font-size: 0.85em;
	}

	.setup-steps {
		display: flex;
		flex-direction: column;
		gap: 12px;
		margin: 16px 0;
		text-align: left;
		max-width: 400px;
	}

	.step {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 12px;
		border-radius: var(--radius);
		background: var(--bg-secondary);
		border: 1px solid var(--border);
	}

	.step strong {
		color: var(--text-primary);
	}

	.step span {
		font-size: 0.9em;
		color: var(--text-secondary);
	}

	.refresh-btn {
		margin-top: 12px;
		padding: 8px 20px;
		border-radius: var(--radius);
		background: var(--accent);
		color: white;
		font-size: 0.9rem;
		cursor: pointer;
		transition: background var(--transition);
	}

	.refresh-btn:hover {
		background: var(--accent-hover);
	}

	.refresh-btn.secondary {
		background: transparent;
		color: var(--text-primary);
		border: 1px solid var(--border);
	}

	.refresh-btn.secondary:hover {
		background: var(--bg-hover);
	}

	.welcome-actions {
		display: flex;
		gap: 10px;
		margin-top: 12px;
	}

	.spinner {
		width: 24px;
		height: 24px;
		border: 3px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
		margin-top: 8px;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.download-progress {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-top: 12px;
		width: 100%;
		max-width: 300px;
	}

	.download-bar {
		flex: 1;
		height: 6px;
		border-radius: 3px;
		background: var(--bg-tertiary);
		overflow: hidden;
	}

	.download-fill {
		height: 100%;
		border-radius: 3px;
		background: var(--accent);
		transition: width 0.5s ease;
	}

	.download-pct {
		font-size: 0.85em;
		color: var(--text-secondary);
		min-width: 36px;
		text-align: right;
	}

	.error-text {
		color: var(--text-primary);
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		padding: 12px 16px;
		border-radius: var(--radius);
		max-width: 500px;
		font-size: 0.9em;
	}

	.error-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 10px 16px;
		margin-top: 8px;
		border-radius: var(--radius);
		background: var(--error-bg);
		border: 1px solid var(--error-border);
		color: var(--error-text);
		font-size: 0.9em;
		max-width: var(--content-max-width);
		width: 100%;
		align-self: center;
	}

	.dismiss-btn {
		padding: 4px 10px;
		border-radius: var(--radius);
		background: transparent;
		border: 1px solid var(--error-border);
		color: var(--error-text);
		font-size: 0.8em;
		cursor: pointer;
		white-space: nowrap;
	}

	.dismiss-btn:hover {
		background: var(--error-bg-hover);
	}

	.manual-hint {
		font-size: 0.85em;
		color: var(--text-muted);
		margin-top: 16px;
	}

	.link-btn {
		color: var(--accent);
		font-size: inherit;
		text-decoration: underline;
		cursor: pointer;
		padding: 0;
	}

	.link-btn:hover {
		color: var(--accent-hover);
	}
</style>
