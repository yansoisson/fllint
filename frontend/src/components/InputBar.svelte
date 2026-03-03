<script lang="ts">
	import ImagePreview from './ImagePreview.svelte';
	import { sendMessage, getIsStreaming, addPendingImage, getPendingImages, getEngineStatus } from '$lib/stores.svelte';

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

	async function handleSubmit() {
		const text = inputText.trim();
		if ((!text && getPendingImages().length === 0) || getIsStreaming()) return;

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
		<ImagePreview />
		<div class="input-row">
			{#if getEngineStatus()?.has_vision}
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
				placeholder={getIsStreaming() ? 'Waiting for response...' : 'Message Fllint...'}
				disabled={getIsStreaming()}
				rows={1}
			></textarea>
			<button
				class="send-btn"
				onclick={handleSubmit}
				disabled={getIsStreaming() || (!inputText.trim() && getPendingImages().length === 0)}
				title="Send message"
				aria-label="Send message"
			>
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
					<line x1="12" y1="19" x2="12" y2="5" />
					<polyline points="5 12 12 5 19 12" />
				</svg>
			</button>
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
</style>
