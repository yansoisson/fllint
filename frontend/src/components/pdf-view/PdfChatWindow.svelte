<script lang="ts">
	import MessageBubble from '$components/MessageBubble.svelte';
	import {
		getPdfMessages,
		getPdfIsStreaming,
		getPdfStreamingContent,
		getPdfStreamingReasoning,
		getPdfThinkingDuration,
		getPdfToolStatus,
		getPdfChatError,
		getCurrentPage,
		getPdfPageCount
	} from '$lib/pdfViewStore.svelte';

	let chatContainer: HTMLDivElement;
	let hasMessages = $derived(getPdfMessages().length > 0 || getPdfIsStreaming());

	// Auto-scroll on new content
	$effect(() => {
		const _ = getPdfMessages();
		const __ = getPdfStreamingContent();
		const ___ = getPdfStreamingReasoning();
		if (chatContainer) {
			const el = chatContainer;
			requestAnimationFrame(() => {
				if (el) el.scrollTop = el.scrollHeight;
			});
		}
	});
</script>

<div class="pdf-chat-window" class:has-messages={hasMessages} bind:this={chatContainer}>
	{#if !hasMessages}
		<div class="empty-state">
			<p class="hint">Ask questions about your PDF. Page {getCurrentPage()} of {getPdfPageCount()} is in view.</p>
		</div>
	{/if}

	{#each getPdfMessages() as message}
		<MessageBubble {message} />
	{/each}

	{#if getPdfIsStreaming() && getPdfToolStatus()}
		<div class="tool-status">
			<span class="tool-spinner"></span>
			{getPdfToolStatus() === 'fetching' ? 'Fetching...' : 'Searching...'}
		</div>
	{/if}

	{#if getPdfIsStreaming() && (getPdfStreamingContent() || getPdfStreamingReasoning())}
		<MessageBubble message={{
			role: 'assistant',
			content: getPdfStreamingContent(),
			reasoning: getPdfStreamingReasoning() || undefined,
			thinking_duration: getPdfThinkingDuration() ?? undefined
		}} isStreamingMessage={true} />
	{/if}

	{#if getPdfChatError()}
		<div class="error-banner">
			{getPdfChatError()}
		</div>
	{/if}
</div>

<style>
	.pdf-chat-window {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.pdf-chat-window:not(.has-messages) {
		justify-content: center;
		align-items: center;
	}

	.empty-state {
		text-align: center;
		padding: 32px 16px;
	}

	.hint {
		color: var(--text-muted);
		font-size: 0.875rem;
	}

	.error-banner {
		padding: 10px 14px;
		background: var(--error-bg, #fef2f2);
		border: 1px solid var(--error-border, #fecaca);
		border-radius: var(--radius);
		color: var(--error-text, #991b1b);
		font-size: 0.85rem;
	}

	.tool-status {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 14px;
		color: var(--text-secondary);
		font-size: 0.85rem;
	}

	.tool-spinner {
		width: 14px;
		height: 14px;
		border: 2px solid var(--border);
		border-top-color: var(--text-primary);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}
</style>
