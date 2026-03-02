<script lang="ts">
	import MessageBubble from './MessageBubble.svelte';
	import { getMessages, getIsStreaming, getStreamingContent } from '$lib/stores.svelte';

	let chatContainer: HTMLDivElement;

	$effect(() => {
		// Trigger on messages or streaming content changes
		const _ = getMessages();
		const __ = getStreamingContent();
		if (chatContainer) {
			// Use tick-like delay to scroll after DOM update
			requestAnimationFrame(() => {
				chatContainer.scrollTop = chatContainer.scrollHeight;
			});
		}
	});
</script>

<div class="chat-window" bind:this={chatContainer}>
	{#each getMessages() as message}
		<MessageBubble {message} />
	{/each}

	{#if getIsStreaming() && getStreamingContent()}
		<MessageBubble message={{ role: 'assistant', content: getStreamingContent() }} />
	{/if}

	{#if getMessages().length === 0 && !getIsStreaming()}
		<div class="empty">
			<h2>Welcome to Fllint</h2>
			<p>Start a conversation by typing a message below.</p>
		</div>
	{/if}
</div>

<style>
	.chat-window {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
		display: flex;
		flex-direction: column;
	}

	.empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		color: var(--text-muted);
		text-align: center;
	}

	.empty h2 {
		margin-bottom: 8px;
		font-weight: 500;
		color: var(--text-secondary);
		font-size: 1.4rem;
	}
</style>
