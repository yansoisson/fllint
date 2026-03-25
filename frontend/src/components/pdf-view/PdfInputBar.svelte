<script lang="ts">
	import {
		sendPdfMessage,
		getPdfIsStreaming,
		cancelPdfStream,
		cancelPdfQueueItem,
		getPdfQueuePosition,
		isPdfReady,
		getContextStatus,
		getOcrInProgress
	} from '$lib/pdfViewStore.svelte';
	import { isEffectiveModelLoading } from '$lib/stores.svelte';

	let inputText = $state('');
	let textareaEl = $state<HTMLTextAreaElement | null>(null);

	let modelLoading = $derived(isEffectiveModelLoading());
	let ctxStatus = $derived(getContextStatus());
	let canSend = $derived(inputText.trim() && !getPdfIsStreaming() && !modelLoading && isPdfReady());

	function autoResize() {
		if (textareaEl) {
			textareaEl.style.height = 'auto';
			textareaEl.style.height = Math.min(textareaEl.scrollHeight, 160) + 'px';
		}
	}

	async function handleSubmit() {
		if (!canSend) return;
		const text = inputText.trim();
		inputText = '';
		if (textareaEl) textareaEl.style.height = 'auto';
		await sendPdfMessage(text);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			handleSubmit();
		}
	}

	function handleStop() {
		cancelPdfQueueItem();
	}
</script>

<div class="pdf-input-bar">
	<div class="status-row">
		<span class="status-dot" class:ready={ctxStatus.ready} class:busy={!ctxStatus.ready && !ctxStatus.detail}></span>
		<span class="status-label" class:error={!ctxStatus.ready && ctxStatus.detail}>{ctxStatus.label}</span>
		{#if ctxStatus.detail}
			<span class="status-detail">{ctxStatus.detail}</span>
		{/if}
	</div>
	<div class="input-row">
		<textarea
			bind:this={textareaEl}
			bind:value={inputText}
			oninput={autoResize}
			onkeydown={handleKeydown}
			placeholder={ctxStatus.ready ? 'Ask about this PDF...' : 'Waiting for PDF to be ready...'}
			rows="1"
			disabled={modelLoading || !isPdfReady()}
		></textarea>
		{#if getPdfIsStreaming()}
			<button class="stop-btn" onclick={handleStop} title="Stop">
				<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
					<rect x="6" y="6" width="12" height="12" rx="2" />
				</svg>
			</button>
		{:else}
			<button
				class="send-btn"
				onclick={handleSubmit}
				disabled={!canSend}
				title={ctxStatus.ready ? 'Send' : ctxStatus.label}
			>
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<line x1="22" y1="2" x2="11" y2="13" />
					<polygon points="22 2 15 22 11 13 2 9 22 2" />
				</svg>
			</button>
		{/if}
	</div>
	{#if getPdfQueuePosition() !== null && getPdfQueuePosition()! > 0}
		<div class="queue-info">Position in queue: {getPdfQueuePosition()}</div>
	{/if}
</div>

<style>
	.pdf-input-bar {
		padding: 8px 16px 12px;
		border-top: 1px solid var(--border);
		background: var(--bg-primary);
		flex-shrink: 0;
	}

	.status-row {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-bottom: 6px;
		font-size: 0.7rem;
	}

	.status-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		flex-shrink: 0;
		background: var(--text-muted);
	}

	.status-dot.ready {
		background: #22c55e;
	}

	.status-dot.busy {
		background: #f59e0b;
		animation: pulse 1.2s ease-in-out infinite;
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.4; }
	}

	.status-label {
		color: var(--text-muted);
		font-weight: 500;
	}

	.status-label.error {
		color: #e74c3c;
	}

	.status-detail {
		color: var(--text-muted);
	}

	.input-row {
		display: flex;
		align-items: flex-end;
		gap: 8px;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 8px 12px;
	}

	textarea {
		flex: 1;
		border: none;
		background: transparent;
		resize: none;
		font-family: inherit;
		font-size: 0.875rem;
		line-height: 1.5;
		color: var(--text-primary);
		outline: none;
		min-height: 24px;
		max-height: 160px;
	}

	textarea::placeholder {
		color: var(--text-muted);
	}

	textarea:disabled {
		opacity: 0.5;
	}

	.send-btn, .stop-btn {
		width: 32px;
		height: 32px;
		border-radius: 8px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		cursor: pointer;
		transition: all 0.15s;
	}

	.send-btn {
		background: var(--text-primary);
		color: var(--bg-primary);
	}

	.send-btn:disabled {
		opacity: 0.3;
		cursor: default;
	}

	.send-btn:not(:disabled):hover {
		opacity: 0.85;
	}

	.stop-btn {
		background: var(--error-text, #991b1b);
		color: white;
	}

	.stop-btn:hover {
		opacity: 0.85;
	}

	.queue-info {
		font-size: 0.75rem;
		color: var(--text-muted);
		text-align: center;
		margin-top: 4px;
	}
</style>
