<script lang="ts">
	import ImagePreview from './ImagePreview.svelte';
	import { sendMessage, getIsStreaming, cancelStream, cancelQueueItem, addPendingImage, getPendingImages, getQueuePosition, isEffectiveModelLoading, getEffectiveModelId, getModels } from '$lib/stores.svelte';

	let inputText = $state('');
	let fileInput: HTMLInputElement;
	let textareaEl: HTMLTextAreaElement;
	let isDragOver = $state(false);

	function autoResize() {
		if (textareaEl) {
			textareaEl.style.height = 'auto';
			textareaEl.style.height = Math.min(textareaEl.scrollHeight, 160) + 'px';
		}
	}

	let modelLoading = $derived(isEffectiveModelLoading());
	let canAttachImage = $derived.by(() => {
		const m = getModels().find((m) => m.id === getEffectiveModelId());
		return m?.vision || m?.external;
	});

	async function handleSubmit() {
		const text = inputText.trim();
		if ((!text && getPendingImages().length === 0) || getIsStreaming() || modelLoading) return;

		inputText = '';
		if (textareaEl) {
			textareaEl.style.height = 'auto';
		}
		await sendMessage(text);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			handleSubmit();
		}
	}

	function handleFileChange(e: Event) {
		const target = e.target as HTMLInputElement;
		const files = target.files;
		if (files) {
			for (const file of files) {
				if (file.type.startsWith('image/')) {
					addPendingImage(file);
				}
			}
		}
		target.value = '';
	}

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
		isDragOver = true;
	}

	function handleDragLeave() {
		isDragOver = false;
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		isDragOver = false;
		const files = e.dataTransfer?.files;
		if (files) {
			for (const file of files) {
				if (file.type.startsWith('image/')) {
					addPendingImage(file);
				}
			}
		}
	}
</script>

<div class="input-bar">
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="input-card"
		class:drag-over={isDragOver}
		ondragover={handleDragOver}
		ondragleave={handleDragLeave}
		ondrop={handleDrop}
	>
		{#if modelLoading}
			<div class="loading-indicator">
				<span class="loading-spinner-small"></span>
				<span class="loading-text">Loading model...</span>
			</div>
		{:else if getQueuePosition() !== null && getQueuePosition()! > 0}
			<div class="queue-indicator">
				<span class="queue-text">Position {getQueuePosition()} in queue...</span>
				<button class="queue-cancel-btn" onclick={cancelQueueItem} title="Cancel" aria-label="Cancel queue item">
					Cancel
				</button>
			</div>
		{:else if getQueuePosition() === 0}
			<div class="queue-indicator">
				<span class="queue-text">Processing...</span>
			</div>
		{/if}
		<ImagePreview />
		<div class="input-row">
			{#if canAttachImage}
				<button class="attach-btn" onclick={() => fileInput.click()} title="Attach image" aria-label="Attach image">
					<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<line x1="12" y1="5" x2="12" y2="19" />
						<line x1="5" y1="12" x2="19" y2="12" />
					</svg>
				</button>
				<input type="file" accept="image/*" multiple bind:this={fileInput} onchange={handleFileChange} hidden />
			{/if}
			<textarea
				bind:this={textareaEl}
				bind:value={inputText}
				oninput={autoResize}
				onkeydown={handleKeydown}
				placeholder={modelLoading ? "Waiting for model to load..." : "Message Fllint..."}
				disabled={modelLoading}
				rows={1}
			></textarea>
			{#if getIsStreaming()}
				<button
					class="stop-btn"
					onclick={cancelStream}
					title="Stop generating"
					aria-label="Stop generating"
				>
					<svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
						<rect x="4" y="4" width="16" height="16" rx="2" />
					</svg>
				</button>
			{:else}
				<button
					class="send-btn"
					onclick={handleSubmit}
					disabled={modelLoading || (!inputText.trim() && getPendingImages().length === 0)}
					title="Send message"
					aria-label="Send message"
				>
					<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
						<line x1="12" y1="19" x2="12" y2="5" />
						<polyline points="5 12 12 5 19 12" />
					</svg>
				</button>
			{/if}
		</div>
	</div>
</div>

<style>
	.input-bar {
		padding: 12px 24px 24px;
		display: flex;
		justify-content: center;
		background: var(--bg-primary);
	}

	.input-card {
		width: 100%;
		max-width: var(--input-max-width);
		background: var(--bg-primary);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-md);
		padding: 8px 8px 8px 4px;
		display: flex;
		flex-direction: column;
		transition: border-color var(--transition), background var(--transition);
	}

	.input-card.drag-over {
		border-color: var(--accent);
		background: var(--bg-secondary);
	}

	.input-row {
		display: flex;
		align-items: flex-end;
		gap: 4px;
	}

	textarea {
		flex: 1;
		resize: none;
		padding: 8px 8px;
		border: none;
		background: transparent;
		outline: none;
		min-height: 24px;
		max-height: 160px;
		line-height: 1.5;
		font-size: 0.9375rem;
	}

	textarea::placeholder {
		color: var(--text-muted);
	}

	textarea:disabled {
		opacity: 0.6;
	}

	.attach-btn {
		width: 36px;
		height: 36px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-secondary);
		transition: all var(--transition);
		flex-shrink: 0;
	}

	.attach-btn:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.send-btn {
		width: 36px;
		height: 36px;
		border-radius: 50%;
		background: var(--accent);
		color: white;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: all var(--transition);
		flex-shrink: 0;
	}

	.send-btn:hover:not(:disabled) {
		background: var(--accent-hover);
	}

	.send-btn:disabled {
		background: var(--bg-tertiary);
		color: var(--text-muted);
		cursor: not-allowed;
	}

	.stop-btn {
		width: 36px;
		height: 36px;
		border-radius: 50%;
		background: var(--text-secondary);
		color: white;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: all var(--transition);
		flex-shrink: 0;
	}

	.stop-btn:hover {
		background: var(--text-primary);
	}

	.queue-indicator {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 6px 12px;
		background: var(--bg-secondary);
		border-radius: var(--radius-md, 8px);
		margin-bottom: 4px;
	}

	.queue-text {
		font-size: 0.8125rem;
		color: var(--text-secondary);
	}

	.queue-cancel-btn {
		font-size: 0.8125rem;
		color: var(--text-secondary);
		padding: 2px 8px;
		border-radius: var(--radius-sm, 4px);
		transition: all var(--transition);
	}

	.queue-cancel-btn:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.loading-indicator {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 12px;
		background: var(--bg-secondary);
		border-radius: var(--radius-md, 8px);
		margin-bottom: 4px;
	}

	.loading-spinner-small {
		width: 14px;
		height: 14px;
		border: 2px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.loading-text {
		font-size: 0.8125rem;
		color: var(--text-secondary);
	}
</style>
