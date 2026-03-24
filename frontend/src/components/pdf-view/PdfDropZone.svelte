<script lang="ts">
	import { loadPdf, getIsProcessing, getPdfChatError } from '$lib/pdfViewStore.svelte';

	let isDragOver = $state(false);
	let fileInput = $state<HTMLInputElement | null>(null);

	function handleFile(file: File) {
		if (file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')) {
			loadPdf(file);
		}
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		isDragOver = false;
		const file = e.dataTransfer?.files[0];
		if (file) handleFile(file);
	}

	function onDragOver(e: DragEvent) {
		e.preventDefault();
		isDragOver = true;
	}

	function onDragLeave() {
		isDragOver = false;
	}

	function onFileChange(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (file) handleFile(file);
	}

	function openFilePicker() {
		fileInput?.click();
	}
</script>

<div
	class="drop-zone"
	class:drag-over={isDragOver}
	ondrop={onDrop}
	ondragover={onDragOver}
	ondragleave={onDragLeave}
>
	{#if getIsProcessing()}
		<div class="loading">
			<div class="spinner"></div>
			<p>Extracting text from PDF...</p>
		</div>
	{:else}
		<svg class="icon" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
			<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
			<polyline points="14 2 14 8 20 8" />
			<line x1="16" y1="13" x2="8" y2="13" />
			<line x1="16" y1="17" x2="8" y2="17" />
			<polyline points="10 9 9 9 8 9" />
		</svg>
		{#if getPdfChatError()}
			<p class="error">{getPdfChatError()}</p>
		{/if}
		<p class="label">Drop a PDF here or click to select</p>
		<button class="select-btn" onclick={openFilePicker}>Choose PDF</button>
		<input
			bind:this={fileInput}
			type="file"
			accept=".pdf,application/pdf"
			onchange={onFileChange}
			class="hidden"
		/>
	{/if}
</div>

<style>
	.drop-zone {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 16px;
		margin: 32px;
		border: 2px dashed var(--border);
		border-radius: 16px;
		transition: all 0.2s;
		cursor: pointer;
	}

	.drop-zone.drag-over {
		border-color: var(--accent, #007aff);
		background: var(--bg-hover);
	}

	.icon {
		color: var(--text-muted);
	}

	.error {
		color: #e74c3c;
		font-size: 0.85rem;
		padding: 6px 12px;
		background: rgba(231, 76, 60, 0.08);
		border-radius: 6px;
	}

	.label {
		color: var(--text-secondary);
		font-size: 0.95rem;
	}

	.select-btn {
		padding: 8px 20px;
		border-radius: var(--radius);
		background: var(--text-primary);
		color: var(--bg-primary);
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		transition: opacity 0.15s;
	}

	.select-btn:hover {
		opacity: 0.85;
	}

	.hidden {
		display: none;
	}

	.loading {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		color: var(--text-secondary);
	}

	.spinner {
		width: 32px;
		height: 32px;
		border: 3px solid var(--border);
		border-top-color: var(--text-primary);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}
</style>
