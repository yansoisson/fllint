<script lang="ts">
	import type { ChatMessage } from '$lib/types';

	let { message }: { message: ChatMessage } = $props();
</script>

<div class="bubble {message.role}">
	<div class="role">{message.role === 'user' ? 'You' : 'Fllint'}</div>
	<div class="content">{message.content}</div>
</div>

<style>
	.bubble {
		padding: 12px 16px;
		border-radius: var(--radius);
		max-width: 80%;
		margin-bottom: 12px;
		line-height: 1.6;
		white-space: pre-wrap;
		word-wrap: break-word;
	}

	.user {
		background: var(--user-bubble);
		align-self: flex-end;
		margin-left: auto;
	}

	.assistant {
		background: var(--assistant-bubble);
		border: 1px solid var(--border);
		align-self: flex-start;
	}

	.role {
		font-size: 0.75rem;
		color: var(--text-muted);
		margin-bottom: 4px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
</style>
