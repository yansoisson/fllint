<script lang="ts">
	import { getPendingDocuments, removePendingDocument } from '$lib/stores.svelte';

	function getFileIcon(name: string): string {
		const ext = name.split('.').pop()?.toLowerCase() ?? '';
		if (ext === 'pdf') return 'PDF';
		if (ext === 'docx') return 'DOC';
		return 'TXT';
	}

	function formatSize(bytes: number): string {
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
		return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
	}
</script>

{#if getPendingDocuments().length > 0}
	<div class="doc-previews">
		{#each getPendingDocuments() as doc, i}
			<div class="doc-chip">
				<span class="doc-badge">{getFileIcon(doc.name)}</span>
				<span class="doc-name">{doc.name}</span>
				<span class="doc-size">{formatSize(doc.file.size)}</span>
				<button class="remove" onclick={() => removePendingDocument(i)}>&times;</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.doc-previews {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		margin: 4px 8px 4px 12px;
	}

	.doc-chip {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 10px;
		font-size: 0.8125rem;
		max-width: 240px;
		position: relative;
	}

	.doc-badge {
		font-size: 0.6875rem;
		font-weight: 600;
		color: var(--accent);
		background: var(--bg-tertiary);
		padding: 1px 5px;
		border-radius: 4px;
		flex-shrink: 0;
	}

	.doc-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-primary);
	}

	.doc-size {
		color: var(--text-muted);
		font-size: 0.75rem;
		flex-shrink: 0;
	}

	.remove {
		width: 18px;
		height: 18px;
		border-radius: 50%;
		background: var(--text-secondary);
		color: white;
		font-size: 11px;
		display: flex;
		align-items: center;
		justify-content: center;
		line-height: 1;
		flex-shrink: 0;
		transition: background var(--transition);
	}

	.remove:hover {
		background: var(--text-primary);
	}
</style>
