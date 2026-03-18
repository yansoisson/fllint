<script lang="ts">
	import { getPendingDocuments, removePendingDocument, openOcrPopup } from '$lib/stores.svelte';

	function getFileIcon(name: string): string {
		const ext = name.split('.').pop()?.toLowerCase() ?? '';
		if (ext === 'pdf') return 'PDF';
		if (ext === 'docx') return 'DOC';
		return 'TXT';
	}

	function isPdf(name: string): boolean {
		return name.toLowerCase().endsWith('.pdf');
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
			<div class="doc-chip" class:has-ocr={!!doc.ocrText} class:processing={doc.ocrProcessing}>
				<span class="doc-badge">{getFileIcon(doc.name)}</span>
				<span class="doc-name">{doc.name}</span>
				<span class="doc-size">{formatSize(doc.file.size)}</span>
				{#if doc.ocrText}
					<span class="ocr-badge">OCR</span>
				{/if}
				{#if isPdf(doc.name)}
					<button class="ocr-btn" class:active={doc.ocrProcessing} title={doc.ocrProcessing ? 'OCR in progress — click to view' : 'Extract text with OCR'} onclick={() => openOcrPopup(i)}>
						{#if doc.ocrProcessing}
							<span class="processing-spinner-small"></span>
						{:else}
							<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
								<path d="M12 20h9" />
								<path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
							</svg>
						{/if}
					</button>
				{/if}
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
		max-width: 280px;
		position: relative;
		transition: all var(--transition);
	}

	.doc-chip.has-ocr {
		border-color: var(--accent);
	}

	.doc-chip.processing {
		opacity: 0.7;
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

	.ocr-badge {
		font-size: 0.625rem;
		font-weight: 600;
		color: white;
		background: var(--accent);
		padding: 1px 4px;
		border-radius: 3px;
		flex-shrink: 0;
	}

	.ocr-btn {
		width: 20px;
		height: 20px;
		border-radius: 4px;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-muted);
		flex-shrink: 0;
		opacity: 0;
		transition: all var(--transition);
	}

	.doc-chip:hover .ocr-btn,
	.ocr-btn.active {
		opacity: 1;
	}

	.ocr-btn:hover {
		background: var(--bg-hover);
		color: var(--accent);
	}

	.processing-spinner-small {
		width: 10px;
		height: 10px;
		border: 1.5px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
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
