<script lang="ts">
	import type { ChatMessage } from '$lib/types';
	import { renderMarkdown } from '$lib/markdown';
	import ThinkingBlock from './ThinkingBlock.svelte';

	let { message, isStreamingMessage = false }: { message: ChatMessage; isStreamingMessage?: boolean } = $props();
</script>

<div class="message {message.role}">
	{#if message.role === 'user'}
		{#if message.images?.length}
			<div class="images">
				{#each message.images as src}
					<!-- svelte-ignore a11y_img_redundant_alt -->
					<img {src} alt="Attached image" class="chat-image" />
				{/each}
			</div>
		{/if}
		{#if message.content}
			<div class="content">{message.content}</div>
		{/if}
	{:else}
		{#if message.reasoning}
			<ThinkingBlock reasoning={message.reasoning} duration={message.thinking_duration ?? null} isStreaming={isStreamingMessage} />
		{/if}
		{#if message.content}
			<div class="content prose">{@html renderMarkdown(message.content)}</div>
		{/if}
	{/if}
</div>

<style>
	.message {
		max-width: var(--content-max-width);
		margin-bottom: 16px;
		line-height: 1.6;
		word-wrap: break-word;
		font-size: 0.9375rem;
		width: 100%;
	}

	.user {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
	}

	.user .content {
		background: var(--user-bubble);
		padding: 10px 16px;
		border-radius: 20px;
		max-width: 85%;
		white-space: pre-wrap;
	}

	.images {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		justify-content: flex-end;
		margin-bottom: 8px;
	}

	.chat-image {
		max-height: 200px;
		max-width: 300px;
		border-radius: 12px;
		border: 1px solid var(--border);
		object-fit: cover;
	}

	.assistant {
		align-self: flex-start;
	}

	.assistant .content {
		padding: 4px 0;
		color: var(--text-primary);
	}

	/* Markdown prose styles for assistant messages */
	.prose :global(p) {
		margin: 0 0 12px 0;
	}

	.prose :global(p:last-child) {
		margin-bottom: 0;
	}

	.prose :global(strong) {
		font-weight: 600;
	}

	.prose :global(em) {
		font-style: italic;
	}

	.prose :global(code) {
		background: var(--bg-tertiary);
		padding: 2px 6px;
		border-radius: 4px;
		font-size: 0.85em;
		font-family: 'SF Mono', 'Fira Code', 'Fira Mono', Menlo, Consolas, monospace;
	}

	.prose :global(pre) {
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 16px;
		margin: 12px 0;
		overflow-x: auto;
	}

	.prose :global(pre code) {
		background: none;
		padding: 0;
		font-size: 0.85em;
	}

	.prose :global(ul),
	.prose :global(ol) {
		margin: 8px 0 12px 0;
		padding-left: 24px;
	}

	.prose :global(li) {
		margin-bottom: 4px;
	}

	.prose :global(h1),
	.prose :global(h2),
	.prose :global(h3),
	.prose :global(h4),
	.prose :global(h5),
	.prose :global(h6) {
		font-weight: 600;
		margin: 16px 0 8px 0;
		color: var(--text-primary);
	}

	.prose :global(h1) {
		font-size: 1.5rem;
	}

	.prose :global(h2) {
		font-size: 1.25rem;
	}

	.prose :global(h3) {
		font-size: 1.1rem;
	}
</style>
