<script lang="ts">
	import { page } from '$app/stores';
	import PdfDropZone from '$components/pdf-view/PdfDropZone.svelte';
	import PdfViewer from '$components/pdf-view/PdfViewer.svelte';
	import PdfChatWindow from '$components/pdf-view/PdfChatWindow.svelte';
	import PdfInputBar from '$components/pdf-view/PdfInputBar.svelte';
	import {
		getPdfFile,
		getPdfConversationId,
		loadPdfConversation,
		resetPdfView
	} from '$lib/pdfViewStore.svelte';

	// Load existing conversation if ?conv= param provided
	$effect(() => {
		const convId = $page.url.searchParams.get('conv');
		if (convId && convId !== getPdfConversationId()) {
			loadPdfConversation(convId);
		}
	});

	// Track divider drag
	let splitPercent = $state(50);
	let isDragging = $state(false);
	let containerRef = $state<HTMLDivElement | null>(null);

	function onDividerMouseDown(e: MouseEvent) {
		e.preventDefault();
		isDragging = true;
	}

	function onMouseMove(e: MouseEvent) {
		if (!isDragging || !containerRef) return;
		const rect = containerRef.getBoundingClientRect();
		const pct = ((e.clientX - rect.left) / rect.width) * 100;
		splitPercent = Math.max(20, Math.min(80, pct));
	}

	function onMouseUp() {
		isDragging = false;
	}
</script>

<svelte:window onmousemove={onMouseMove} onmouseup={onMouseUp} />

<div class="pdf-view-page">
	{#if !getPdfFile()}
		<PdfDropZone />
	{:else}
		<div
			class="split-view"
			class:dragging={isDragging}
			bind:this={containerRef}
		>
			<div class="panel pdf-panel" style="width: {splitPercent}%">
				<PdfViewer />
			</div>
			<div
				class="divider"
				role="separator"
				onmousedown={onDividerMouseDown}
			></div>
			<div class="panel chat-panel" style="width: {100 - splitPercent}%">
				<PdfChatWindow />
				<PdfInputBar />
			</div>
		</div>
	{/if}
</div>

<style>
	.pdf-view-page {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow: hidden;
	}

	.split-view {
		flex: 1;
		display: flex;
		min-height: 0;
		overflow: hidden;
	}

	.split-view.dragging {
		cursor: col-resize;
		user-select: none;
	}

	.panel {
		display: flex;
		flex-direction: column;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
	}

	.chat-panel {
		border-left: 1px solid var(--border);
	}

	.divider {
		width: 4px;
		flex-shrink: 0;
		cursor: col-resize;
		background: transparent;
		transition: background 0.15s;
		position: relative;
	}

	.divider:hover,
	.dragging .divider {
		background: var(--accent, #007aff);
	}
</style>
