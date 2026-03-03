<script lang="ts">
	import MessageBubble from './MessageBubble.svelte';
	import {
		getMessages,
		getIsStreaming,
		getStreamingContent,
		getEngineStatus,
		getChatError,
		clearChatError,
		refreshModels
	} from '$lib/stores.svelte';

	let chatContainer: HTMLDivElement;
	let hasMessages = $derived(getMessages().length > 0 || getIsStreaming());

	$effect(() => {
		const _ = getMessages();
		const __ = getStreamingContent();
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

	{#if getIsStreaming() && getStreamingContent()}
		<MessageBubble message={{ role: 'assistant', content: getStreamingContent() }} />
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
			{:else if !getEngineStatus()?.has_models}
				<h2>Almost there</h2>
				<p>
					llama-server is ready, but no models found.
					Place a .gguf model file in the <code>models/</code> folder.
				</p>
				<button class="refresh-btn" onclick={refreshModels}>Refresh</button>
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
</style>
