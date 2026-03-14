<script lang="ts">
	import {
		getConversations,
		getActiveConversationId,
		deleteConversation,
		navigateToNewConversation,
		getSidebarOpen,
		getAppVersion
	} from '$lib/stores.svelte';

	let confirmDeleteId = $state<string | null>(null);
	let confirmTimeout: ReturnType<typeof setTimeout> | null = null;

	function handleDelete(id: string, e: MouseEvent) {
		e.preventDefault();
		e.stopPropagation();
		if (confirmDeleteId === id) {
			deleteConversation(id);
			confirmDeleteId = null;
			if (confirmTimeout) clearTimeout(confirmTimeout);
		} else {
			confirmDeleteId = id;
			if (confirmTimeout) clearTimeout(confirmTimeout);
			confirmTimeout = setTimeout(() => { confirmDeleteId = null; }, 3000);
		}
	}
</script>

<aside class="sidebar" class:open={getSidebarOpen()}>
	<div class="header">
		<span class="brand">Fllint</span>
		<button class="new-btn" onclick={navigateToNewConversation} title="New chat">
			<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
				<path d="M12 20h9" />
				<path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
			</svg>
		</button>
	</div>
	<div class="list">
		{#each getConversations() as conv}
			<a
				class="conv-item"
				class:active={getActiveConversationId() === conv.id}
				href="/chat/{conv.id}"
			>
				<span class="title">{conv.title}</span>
				<button
					class="delete"
					class:confirm={confirmDeleteId === conv.id}
					onclick={(e) => handleDelete(conv.id, e)}
				>
					{confirmDeleteId === conv.id ? '?' : '\u00d7'}
				</button>
			</a>
		{/each}

		{#if getConversations().length === 0}
			<div class="empty">No conversations yet</div>
		{/if}
	</div>
	{#if getAppVersion()}
		<div class="version-badge">v{getAppVersion()}</div>
	{/if}
</aside>

<style>
	.sidebar {
		width: var(--sidebar-width);
		background: var(--bg-secondary);
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
		height: var(--header-height);
	}

	.brand {
		font-size: 1rem;
		font-weight: 700;
		color: var(--text-primary);
		letter-spacing: -0.01em;
	}

	.new-btn {
		width: 36px;
		height: 36px;
		border-radius: var(--radius);
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-secondary);
		transition: all var(--transition);
	}

	.new-btn:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.list {
		flex: 1;
		overflow-y: auto;
		padding: 4px 8px;
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
		margin-bottom: 1px;
		cursor: pointer;
		text-decoration: none;
		color: inherit;
	}

	.conv-item:hover {
		background: var(--bg-hover);
	}

	.conv-item.active {
		background: var(--bg-tertiary);
	}

	.title {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
		font-size: 0.875rem;
		color: var(--text-primary);
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
		color: #e74c3c;
		background: rgba(231, 76, 60, 0.08);
	}

	.delete.confirm {
		opacity: 1;
		color: #e74c3c;
		background: rgba(231, 76, 60, 0.12);
		font-weight: 700;
	}

	.empty {
		padding: 24px 16px;
		text-align: center;
		color: var(--text-muted);
		font-size: 0.875rem;
	}

	.version-badge {
		padding: 8px 16px;
		color: var(--text-muted);
		font-size: 0.7rem;
	}
</style>
