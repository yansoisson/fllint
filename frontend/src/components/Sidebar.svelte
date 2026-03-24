<script lang="ts">
	import { page } from '$app/stores';
	import { APPS } from '$lib/apps';
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

	function convHref(conv: { id: string; app_type?: string }): string {
		if (conv.app_type === 'pdf-view') {
			return `/apps/pdf-view?conv=${conv.id}`;
		}
		return `/chat/${conv.id}`;
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

	<div class="apps-section">
		<span class="section-label">Apps</span>
		{#each APPS as app}
			<a
				class="app-item"
				class:active={$page.url.pathname.startsWith(app.route)}
				href={app.route}
			>
				{#if app.icon === 'document'}
					<svg class="app-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
						<polyline points="14 2 14 8 20 8" />
						<line x1="16" y1="13" x2="8" y2="13" />
						<line x1="16" y1="17" x2="8" y2="17" />
						<polyline points="10 9 9 9 8 9" />
					</svg>
				{/if}
				<span>{app.name}</span>
			</a>
		{/each}
	</div>

	<div class="list">
		{#each getConversations() as conv}
			<a
				class="conv-item"
				class:active={getActiveConversationId() === conv.id}
				href={convHref(conv)}
			>
				{#if conv.app_type === 'pdf-view'}
					<span class="pdf-badge">PDF</span>
				{/if}
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

	.apps-section {
		padding: 4px 8px 0;
	}

	.section-label {
		display: block;
		padding: 4px 12px 6px;
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
	}

	.app-item {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		border-radius: var(--radius);
		font-size: 0.875rem;
		color: var(--text-primary);
		text-decoration: none;
		transition: background var(--transition);
		cursor: pointer;
	}

	.app-item:hover {
		background: var(--bg-hover);
	}

	.app-item.active {
		background: var(--bg-tertiary);
	}

	.app-icon {
		flex-shrink: 0;
		color: var(--text-secondary);
	}

	.list {
		flex: 1;
		overflow-y: auto;
		padding: 4px 8px;
		border-top: 1px solid var(--border);
		margin-top: 4px;
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

	.pdf-badge {
		flex-shrink: 0;
		font-size: 0.6rem;
		font-weight: 700;
		padding: 1px 4px;
		border-radius: 3px;
		background: var(--bg-tertiary);
		color: var(--text-secondary);
		margin-right: 6px;
		text-transform: uppercase;
		letter-spacing: 0.03em;
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
