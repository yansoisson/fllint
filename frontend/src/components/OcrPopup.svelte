<script lang="ts">
	import { getOcrPopup, getOcrProgress, closeOcrPopup, startOcrProcessing, cancelOcrProcessing, openSettingsToTab, isOcrEnabled } from '$lib/stores.svelte';
	import { parsePageRange } from '$lib/pdf';

	let pageInput = $state('all');
	let starting = $state(false);

	let popup = $derived(getOcrPopup());
	let progress = $derived(getOcrProgress());
	let isProcessing = $derived(progress?.status === 'processing');
	let ocrAvailable = $derived(isOcrEnabled());

	async function handleStart() {
		if (!popup || starting) return;
		starting = true;
		const pages = parsePageRange(pageInput, popup.pageCount);
		if (pages.length === 0) {
			starting = false;
			return;
		}
		await startOcrProcessing(pages);
		starting = false;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (!popup) return;
		if (e.key === 'Escape') {
			if (isProcessing) {
				closeOcrPopup(); // Just hides popup, processing continues
			} else {
				closeOcrPopup();
			}
		}
	}

	function goToSettings() {
		closeOcrPopup();
		openSettingsToTab('helper');
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if popup}
	<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
	<div class="overlay" onclick={() => closeOcrPopup()}>
		<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
		<div class="popup" onclick={(e) => e.stopPropagation()}>
			<h3 class="popup-title">OCR — {popup.filename}</h3>
			<p class="popup-desc">{popup.pageCount} page{popup.pageCount !== 1 ? 's' : ''} detected</p>

			{#if !ocrAvailable}
				<div class="disabled-section">
					<p class="disabled-text">OCR is not configured. Select an OCR model in Settings to extract text from scanned PDFs.</p>
					<div class="popup-actions">
						<button class="action-btn secondary" onclick={() => closeOcrPopup()}>Cancel</button>
						<button class="action-btn primary" onclick={goToSettings}>Open Settings</button>
					</div>
				</div>
			{:else if isProcessing && progress}
				<div class="progress-section">
					<div class="progress-bar">
						<div
							class="progress-fill"
							style="width: {((progress.done_pages / progress.total_pages) * 100).toFixed(0)}%"
						></div>
					</div>
					<p class="progress-text">Processing page {progress.done_pages + 1} of {progress.total_pages}...</p>
					<button class="cancel-btn" onclick={cancelOcrProcessing}>Cancel</button>
				</div>
			{:else}
				<div class="field">
					<label class="field-label" for="ocr-pages">Pages</label>
					<input
						class="page-input"
						id="ocr-pages"
						type="text"
						bind:value={pageInput}
						placeholder="e.g. 1-3, 5, 7-10 or all"
					/>
					<p class="field-hint">Enter page numbers, ranges, or "all"</p>
				</div>

				<div class="popup-actions">
					<button class="action-btn secondary" onclick={() => closeOcrPopup()}>Cancel</button>
					<button
						class="action-btn primary"
						onclick={handleStart}
						disabled={starting || parsePageRange(pageInput, popup.pageCount).length === 0}
					>
						{#if starting}
							Preparing...
						{:else}
							Start OCR ({parsePageRange(pageInput, popup.pageCount).length} page{parsePageRange(pageInput, popup.pageCount).length !== 1 ? 's' : ''})
						{/if}
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.4);
		z-index: 300;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.popup {
		background: var(--bg-primary);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 24px;
		width: 400px;
		max-width: 90vw;
		box-shadow: var(--shadow-lg);
		animation: popup-in 0.2s ease;
	}

	@keyframes popup-in {
		from { opacity: 0; transform: scale(0.95); }
		to { opacity: 1; transform: scale(1); }
	}

	.popup-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 4px;
	}

	.popup-desc {
		font-size: 0.85rem;
		color: var(--text-secondary);
		margin-bottom: 16px;
	}

	.field {
		margin-bottom: 16px;
	}

	.field-label {
		display: block;
		font-size: 0.85rem;
		font-weight: 500;
		color: var(--text-secondary);
		margin-bottom: 6px;
	}

	.page-input {
		width: 100%;
		padding: 8px 12px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-input);
		font-size: 0.9rem;
		outline: none;
		transition: border-color var(--transition);
	}

	.page-input:focus {
		border-color: var(--accent);
	}

	.field-hint {
		font-size: 0.75rem;
		color: var(--text-muted);
		margin-top: 4px;
	}

	.popup-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	.action-btn {
		padding: 8px 16px;
		border-radius: var(--radius);
		font-size: 0.85rem;
		font-weight: 500;
		transition: all var(--transition);
	}

	.action-btn.secondary {
		color: var(--text-secondary);
	}

	.action-btn.secondary:hover {
		background: var(--bg-hover);
	}

	.action-btn.primary {
		background: var(--accent);
		color: white;
	}

	.action-btn.primary:hover:not(:disabled) {
		background: var(--accent-hover);
	}

	.action-btn.primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.progress-section {
		text-align: center;
	}

	.progress-bar {
		width: 100%;
		height: 6px;
		background: var(--bg-tertiary);
		border-radius: 3px;
		overflow: hidden;
		margin-bottom: 8px;
	}

	.progress-fill {
		height: 100%;
		background: var(--accent);
		border-radius: 3px;
		transition: width 0.3s ease;
	}

	.progress-text {
		font-size: 0.85rem;
		color: var(--text-secondary);
		margin-bottom: 12px;
	}

	.cancel-btn {
		padding: 6px 16px;
		border-radius: var(--radius);
		font-size: 0.85rem;
		color: var(--text-secondary);
		transition: all var(--transition);
	}

	.cancel-btn:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.disabled-section {
		text-align: center;
	}

	.disabled-text {
		font-size: 0.85rem;
		color: var(--text-secondary);
		margin-bottom: 16px;
		line-height: 1.5;
	}
</style>
