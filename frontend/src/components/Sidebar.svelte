<script lang="ts">
	import {
		getConversations,
		getActiveConversationId,
		selectConversation,
		newConversation,
		deleteConversation,
		getSidebarOpen
	} from '$lib/stores.svelte';
</script>

<aside class="sidebar" class:open={getSidebarOpen()}>
	<div class="header">
		<h2>Chats</h2>
		<button class="new-btn" onclick={newConversation}>+ New</button>
	</div>
	<div class="list">
		{#each getConversations() as conv}
			<div
				class="conv-item"
				class:active={getActiveConversationId() === conv.id}
				role="button"
				tabindex="0"
				onclick={() => selectConversation(conv.id)}
				onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') selectConversation(conv.id); }}
			>
				<span class="title">{conv.title}</span>
				<button
					class="delete"
					onclick={(e: MouseEvent) => {
						e.stopPropagation();
						deleteConversation(conv.id);
					}}
				>
					&times;
				</button>
			</div>
		{/each}

		{#if getConversations().length === 0}
			<div class="empty">No conversations yet</div>
		{/if}
	</div>
</aside>

<style>
	.sidebar {
		width: var(--sidebar-width);
		background: var(--bg-secondary);
		border-right: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		transition: margin-left var(--transition);
		flex-shrink: 0;
	}

	.sidebar:not(.open) {
		margin-left: calc(-1 * var(--sidebar-width));
	}

	.header {
		padding: 12px 16px;
		display: flex;
		justify-content: space-between;
		align-items: center;
		border-bottom: 1px solid var(--border);
		height: var(--header-height);
	}

	.header h2 {
		font-size: 0.95rem;
		font-weight: 600;
	}

	.new-btn {
		padding: 5px 12px;
		border-radius: var(--radius);
		background: var(--accent);
		color: white;
		font-size: 0.8rem;
		font-weight: 600;
		transition: background var(--transition);
	}

	.new-btn:hover {
		background: var(--accent-hover);
	}

	.list {
		flex: 1;
		overflow-y: auto;
		padding: 8px;
	}

	.conv-item {
		width: 100%;
		padding: 10px 12px;
		border-radius: var(--radius);
		text-align: left;
		display: flex;
		justify-content: space-between;
		align-items: center;
		transition: background var(--transition);
		margin-bottom: 2px;
		cursor: pointer;
	}

	.conv-item:hover {
		background: var(--bg-tertiary);
	}

	.conv-item.active {
		background: var(--bg-tertiary);
	}

	.title {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
		font-size: 0.85rem;
	}

	.delete {
		opacity: 0;
		margin-left: 8px;
		color: var(--text-muted);
		font-size: 16px;
		padding: 2px 6px;
		border-radius: 4px;
		transition: all var(--transition);
	}

	.conv-item:hover .delete {
		opacity: 1;
	}

	.delete:hover {
		color: var(--accent);
		background: rgba(233, 69, 96, 0.1);
	}

	.empty {
		padding: 24px 16px;
		text-align: center;
		color: var(--text-muted);
		font-size: 0.85rem;
	}
</style>
