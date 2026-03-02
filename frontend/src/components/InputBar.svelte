<script lang="ts">
	import ImagePreview from './ImagePreview.svelte';
	import { sendMessage, getIsStreaming, setPendingImage, getPendingImage } from '$lib/stores.svelte';
	import { uploadImage } from '$lib/api';

	let inputText = $state('');
	let fileInput: HTMLInputElement;

	async function handleSubmit() {
		const text = inputText.trim();
		if (!text || getIsStreaming()) return;

		if (getPendingImage()) {
			try {
				await uploadImage(getPendingImage()!.file);
			} catch (err) {
				console.error('Image upload failed:', err);
			}
			setPendingImage(null);
		}

		inputText = '';
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
		const file = target.files?.[0];
		if (file && file.type.startsWith('image/')) {
			setPendingImage(file);
		}
		target.value = '';
	}
</script>

<div class="input-bar">
	<ImagePreview />
	<div class="input-row">
		<button class="upload-btn" onclick={() => fileInput.click()} title="Upload image">
			<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
				<circle cx="8.5" cy="8.5" r="1.5" />
				<polyline points="21 15 16 10 5 21" />
			</svg>
		</button>
		<input type="file" accept="image/*" bind:this={fileInput} onchange={handleFileChange} hidden />
		<textarea
			bind:value={inputText}
			onkeydown={handleKeydown}
			placeholder={getIsStreaming() ? 'Waiting for response...' : 'Type a message...'}
			disabled={getIsStreaming()}
			rows={1}
		></textarea>
		<button
			class="send-btn"
			onclick={handleSubmit}
			disabled={getIsStreaming() || !inputText.trim()}
		>
			Send
		</button>
	</div>
</div>

<style>
	.input-bar {
		padding: 12px 24px 16px;
		border-top: 1px solid var(--border);
		background: var(--bg-secondary);
	}

	.input-row {
		display: flex;
		align-items: flex-end;
		gap: 8px;
	}

	textarea {
		flex: 1;
		resize: none;
		padding: 12px 16px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		background: var(--bg-input);
		outline: none;
		min-height: var(--input-height);
		max-height: 120px;
		transition: border-color var(--transition);
		line-height: 1.5;
	}

	textarea:focus {
		border-color: var(--accent);
	}

	textarea:disabled {
		opacity: 0.6;
	}

	.upload-btn {
		width: 44px;
		height: 44px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-secondary);
		transition: all var(--transition);
		flex-shrink: 0;
	}

	.upload-btn:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.send-btn {
		padding: 12px 20px;
		border-radius: var(--radius);
		background: var(--accent);
		color: white;
		font-weight: 600;
		transition: background var(--transition);
		flex-shrink: 0;
		height: 44px;
	}

	.send-btn:hover:not(:disabled) {
		background: var(--accent-hover);
	}

	.send-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
</style>
