<script lang="ts">
	import * as pdfjs from 'pdfjs-dist';
	import { parsePageRange } from '$lib/pdf';
	import {
		getPdfFile,
		getPdfPageCount,
		getCurrentPage,
		setCurrentPage,
		getIsDrawing,
		setIsDrawing,
		getPenColor,
		setPenColor,
		getPenSize,
		setPenSize,
		getAnnotationData,
		saveAnnotation,
		clearAnnotation,
		setCanvasRefs,
		startPdfOcr,
		cancelPdfOcr,
		getOcrInProgress,
		getOcrProgress,
		getIsOcrAvailable
	} from '$lib/pdfViewStore.svelte';

	let pdfDoc = $state<pdfjs.PDFDocumentProxy | null>(null);
	let scale = $state(1.0);
	let pageElements: Map<number, HTMLDivElement> = new Map();
	let renderedPages: Set<number> = new Set();
	let pageInput = $state('1');
	let scrollContainer = $state<HTMLDivElement | null>(null);

	// Drawing state
	let isMouseDown = false;

	const PEN_COLORS = ['#ff0000', '#0066ff', '#00aa00', '#ffaa00', '#000000'];

	// OCR popover
	let showOcrPopover = $state(false);
	let ocrPageInput = $state('all');

	function handleOcrStart() {
		startPdfOcr(ocrPageInput);
		showOcrPopover = false;
	}

	// Svelte action to register page elements
	function pageRef(node: HTMLDivElement, pageNum: number) {
		pageElements.set(pageNum, node);
		return {
			destroy() {
				pageElements.delete(pageNum);
			}
		};
	}

	// Load PDF document
	$effect(() => {
		const file = getPdfFile();
		if (!file) return;

		let cancelled = false;
		(async () => {
			const arrayBuffer = await file.arrayBuffer();
			const doc = await pdfjs.getDocument({ data: new Uint8Array(arrayBuffer) }).promise;
			if (!cancelled) {
				pdfDoc = doc;
				renderedPages = new Set();
			}
		})();

		return () => { cancelled = true; };
	});

	// Fit to width when doc loads
	$effect(() => {
		if (!pdfDoc || !scrollContainer) return;
		let cancelled = false;
		(async () => {
			const page = await pdfDoc!.getPage(1);
			if (cancelled) return;
			const viewport = page.getViewport({ scale: 1.0 });
			const containerWidth = scrollContainer!.clientWidth - 32;
			scale = containerWidth / viewport.width;
		})();
		return () => { cancelled = true; };
	});

	// Render visible pages when scale, doc, or page count changes
	$effect(() => {
		const count = getPdfPageCount();
		if (!pdfDoc || scale <= 0 || count === 0) return;
		// Small delay to let DOM settle after page divs are created
		const t = setTimeout(() => renderVisiblePages(), 100);
		return () => clearTimeout(t);
	});

	// Observe which pages are visible
	$effect(() => {
		if (!scrollContainer || getPdfPageCount() === 0) return;

		const observer = new IntersectionObserver(
			(entries) => {
				let bestPage = getCurrentPage();
				let bestRatio = 0;
				for (const entry of entries) {
					const pageNum = parseInt(entry.target.getAttribute('data-page') ?? '0');
					if (entry.intersectionRatio > bestRatio) {
						bestRatio = entry.intersectionRatio;
						bestPage = pageNum;
					}
				}
				if (bestRatio > 0 && bestPage !== getCurrentPage()) {
					setCurrentPage(bestPage);
					pageInput = String(bestPage);
				}
				renderVisiblePages();
			},
			{
				root: scrollContainer,
				threshold: [0, 0.25, 0.5, 0.75, 1.0]
			}
		);

		const timeout = setTimeout(() => {
			pageElements.forEach((el) => observer.observe(el));
		}, 150);

		return () => {
			clearTimeout(timeout);
			observer.disconnect();
		};
	});

	// Update canvas refs for capture
	$effect(() => {
		const page = getCurrentPage();
		const el = pageElements.get(page);
		if (el) {
			const pdfCanvas = el.querySelector<HTMLCanvasElement>('.pdf-canvas');
			const annotCanvas = el.querySelector<HTMLCanvasElement>('.annot-canvas');
			setCanvasRefs(pdfCanvas, annotCanvas);
		}
	});

	async function renderVisiblePages() {
		if (!pdfDoc || !scrollContainer) return;

		const current = getCurrentPage();
		const start = Math.max(1, current - 2);
		const end = Math.min(getPdfPageCount(), current + 2);

		for (let i = start; i <= end; i++) {
			if (renderedPages.has(i)) continue;
			renderedPages.add(i);
			await renderPage(i);
		}
	}

	async function renderPage(pageNum: number) {
		if (!pdfDoc) return;
		const el = pageElements.get(pageNum);
		if (!el) return;

		const page = await pdfDoc.getPage(pageNum);
		const dpr = window.devicePixelRatio || 1;
		const viewport = page.getViewport({ scale });

		const pdfCanvas = el.querySelector<HTMLCanvasElement>('.pdf-canvas');
		const annotCanvas = el.querySelector<HTMLCanvasElement>('.annot-canvas');
		if (!pdfCanvas || !annotCanvas) return;

		pdfCanvas.style.width = `${viewport.width}px`;
		pdfCanvas.style.height = `${viewport.height}px`;
		annotCanvas.style.width = `${viewport.width}px`;
		annotCanvas.style.height = `${viewport.height}px`;

		pdfCanvas.width = viewport.width * dpr;
		pdfCanvas.height = viewport.height * dpr;
		annotCanvas.width = viewport.width * dpr;
		annotCanvas.height = viewport.height * dpr;

		const ctx = pdfCanvas.getContext('2d')!;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		await page.render({ canvasContext: ctx, canvas: pdfCanvas, viewport }).promise;

		// Restore saved annotations
		const savedData = getAnnotationData().get(pageNum);
		if (savedData) {
			const actx = annotCanvas.getContext('2d')!;
			actx.putImageData(savedData, 0, 0);
		}

		const wrapper = el.querySelector<HTMLDivElement>('.page-canvases');
		if (wrapper) {
			wrapper.style.width = `${viewport.width}px`;
			wrapper.style.height = `${viewport.height}px`;
		}
	}

	// --- Drawing handlers ---
	function getCanvasPoint(e: MouseEvent, canvas: HTMLCanvasElement): { x: number; y: number } {
		const rect = canvas.getBoundingClientRect();
		const dpr = window.devicePixelRatio || 1;
		return {
			x: (e.clientX - rect.left) * dpr,
			y: (e.clientY - rect.top) * dpr
		};
	}

	function onAnnotMouseDown(e: MouseEvent, pageNum: number) {
		if (!getIsDrawing()) return;
		const el = pageElements.get(pageNum);
		const canvas = el?.querySelector<HTMLCanvasElement>('.annot-canvas');
		if (!canvas) return;

		isMouseDown = true;
		const point = getCanvasPoint(e, canvas);

		const ctx = canvas.getContext('2d')!;
		const dpr = window.devicePixelRatio || 1;
		ctx.lineCap = 'round';
		ctx.lineJoin = 'round';
		ctx.strokeStyle = getPenColor();
		ctx.lineWidth = getPenSize() * dpr;
		ctx.beginPath();
		ctx.moveTo(point.x, point.y);
	}

	function onAnnotMouseMove(e: MouseEvent, pageNum: number) {
		if (!isMouseDown || !getIsDrawing()) return;
		const el = pageElements.get(pageNum);
		const canvas = el?.querySelector<HTMLCanvasElement>('.annot-canvas');
		if (!canvas) return;

		const point = getCanvasPoint(e, canvas);
		const ctx = canvas.getContext('2d')!;
		ctx.lineTo(point.x, point.y);
		ctx.stroke();
		ctx.beginPath();
		ctx.moveTo(point.x, point.y);
	}

	function onAnnotMouseUp(_e: MouseEvent, pageNum: number) {
		if (!isMouseDown) return;
		isMouseDown = false;

		const el = pageElements.get(pageNum);
		const canvas = el?.querySelector<HTMLCanvasElement>('.annot-canvas');
		if (!canvas) return;

		const ctx = canvas.getContext('2d')!;
		const data = ctx.getImageData(0, 0, canvas.width, canvas.height);
		saveAnnotation(pageNum, data);
	}

	function handleClearAnnotation() {
		const pageNum = getCurrentPage();
		clearAnnotation(pageNum);
		const el = pageElements.get(pageNum);
		const canvas = el?.querySelector<HTMLCanvasElement>('.annot-canvas');
		if (canvas) {
			const ctx = canvas.getContext('2d')!;
			ctx.clearRect(0, 0, canvas.width, canvas.height);
		}
	}

	function goToPage(num: number) {
		if (num < 1 || num > getPdfPageCount()) return;
		setCurrentPage(num);
		pageInput = String(num);
		const el = pageElements.get(num);
		if (el) {
			el.scrollIntoView({ behavior: 'smooth', block: 'start' });
		}
	}

	function onPageInputKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			const num = parseInt(pageInput);
			if (!isNaN(num)) goToPage(num);
		}
	}
</script>

<div class="pdf-viewer">
	<div class="toolbar">
		<div class="nav-group">
			<button class="tb-btn" onclick={() => goToPage(getCurrentPage() - 1)} disabled={getCurrentPage() <= 1} title="Previous page">
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6" /></svg>
			</button>
			<input
				class="page-input"
				type="text"
				bind:value={pageInput}
				onkeydown={onPageInputKeydown}
				onblur={() => { const n = parseInt(pageInput); if (!isNaN(n)) goToPage(n); }}
			/>
			<span class="page-total">/ {getPdfPageCount()}</span>
			<button class="tb-btn" onclick={() => goToPage(getCurrentPage() + 1)} disabled={getCurrentPage() >= getPdfPageCount()} title="Next page">
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6" /></svg>
			</button>
		</div>

		<div class="draw-group">
			<button
				class="tb-btn"
				class:active={getIsDrawing()}
				onclick={() => setIsDrawing(!getIsDrawing())}
				title="Toggle pen"
			>
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M12 20h9" />
					<path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
				</svg>
			</button>
			{#if getIsDrawing()}
				<div class="color-picker">
					{#each PEN_COLORS as color}
						<button
							class="color-dot"
							class:selected={getPenColor() === color}
							style="background: {color}"
							onclick={() => setPenColor(color)}
						></button>
					{/each}
				</div>
				<select class="size-select" value={String(getPenSize())} onchange={(e) => setPenSize(parseInt((e.target as HTMLSelectElement).value))}>
					<option value="2">Thin</option>
					<option value="3">Medium</option>
					<option value="5">Thick</option>
				</select>
				<button class="tb-btn" onclick={handleClearAnnotation} title="Clear annotations on this page">
					<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<polyline points="3 6 5 6 21 6" />
						<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
					</svg>
				</button>
			{/if}
		</div>

		<div class="ocr-group">
			{#if getOcrInProgress()}
				<div class="ocr-progress">
					<span class="ocr-spinner"></span>
					<span class="ocr-text">OCR {getOcrProgress()?.done ?? 0}/{getOcrProgress()?.total ?? 0}</span>
					<button class="tb-btn" onclick={cancelPdfOcr} title="Cancel OCR">
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
					</button>
				</div>
			{:else}
				<div class="ocr-wrapper">
					<button
						class="tb-btn"
						onclick={() => { showOcrPopover = !showOcrPopover; }}
						disabled={!getIsOcrAvailable()}
						title={getIsOcrAvailable() ? 'OCR pages' : 'OCR not configured — set an OCR model in Settings'}
					>
						<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<rect x="3" y="3" width="18" height="18" rx="2" />
							<path d="M7 8h10" /><path d="M7 12h10" /><path d="M7 16h6" />
						</svg>
					</button>
					{#if showOcrPopover}
						<div class="ocr-popover">
							<label class="ocr-label">Pages to OCR</label>
							<input
								class="ocr-input"
								type="text"
								bind:value={ocrPageInput}
								placeholder="e.g. 1-3, 5 or all"
								onkeydown={(e) => { if (e.key === 'Enter') handleOcrStart(); }}
							/>
							<div class="ocr-actions">
								<button class="ocr-cancel" onclick={() => { showOcrPopover = false; }}>Cancel</button>
								<button
									class="ocr-start"
									onclick={handleOcrStart}
									disabled={parsePageRange(ocrPageInput, getPdfPageCount()).length === 0}
								>
									Start ({parsePageRange(ocrPageInput, getPdfPageCount()).length} pages)
								</button>
							</div>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>

	<div class="pages-container" bind:this={scrollContainer}>
		{#each Array(getPdfPageCount()) as _, i}
			{@const pageNum = i + 1}
			<div
				class="pdf-page"
				data-page={pageNum}
				use:pageRef={pageNum}
			>
				<div class="page-number">{pageNum}</div>
				<div class="page-canvases">
					<canvas class="pdf-canvas"></canvas>
					<canvas
						class="annot-canvas"
						class:drawing={getIsDrawing()}
						onmousedown={(e) => onAnnotMouseDown(e, pageNum)}
						onmousemove={(e) => onAnnotMouseMove(e, pageNum)}
						onmouseup={(e) => onAnnotMouseUp(e, pageNum)}
						onmouseleave={(e) => onAnnotMouseUp(e, pageNum)}
					></canvas>
				</div>
			</div>
		{/each}
	</div>
</div>

<style>
	.pdf-viewer {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow: hidden;
	}

	.toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 6px 12px;
		border-bottom: 1px solid var(--border);
		background: var(--bg-primary);
		gap: 8px;
		flex-shrink: 0;
	}

	.nav-group, .draw-group {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.tb-btn {
		width: 30px;
		height: 30px;
		border-radius: 6px;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-secondary);
		transition: all 0.15s;
		cursor: pointer;
	}

	.tb-btn:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.tb-btn:disabled {
		opacity: 0.3;
		cursor: default;
	}

	.tb-btn.active {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.page-input {
		width: 40px;
		text-align: center;
		padding: 4px;
		border: 1px solid var(--border);
		border-radius: 4px;
		font-size: 0.8rem;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.page-total {
		font-size: 0.8rem;
		color: var(--text-muted);
	}

	.color-picker {
		display: flex;
		gap: 3px;
		margin-left: 4px;
	}

	.color-dot {
		width: 18px;
		height: 18px;
		border-radius: 50%;
		border: 2px solid transparent;
		cursor: pointer;
		transition: border-color 0.15s;
	}

	.color-dot.selected {
		border-color: var(--text-primary);
	}

	.size-select {
		font-size: 0.75rem;
		padding: 2px 4px;
		border: 1px solid var(--border);
		border-radius: 4px;
		background: var(--bg-primary);
		color: var(--text-primary);
		margin-left: 4px;
	}

	.pages-container {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
		background: var(--bg-secondary);
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
	}

	.pdf-page {
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	.page-number {
		font-size: 0.7rem;
		color: var(--text-muted);
		margin-bottom: 4px;
	}

	.page-canvases {
		position: relative;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
		background: white;
	}

	.pdf-canvas {
		display: block;
	}

	.annot-canvas {
		position: absolute;
		top: 0;
		left: 0;
		pointer-events: none;
	}

	.annot-canvas.drawing {
		pointer-events: auto;
		cursor: crosshair;
	}

	.ocr-group {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.ocr-wrapper {
		position: relative;
	}

	.ocr-progress {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	.ocr-spinner {
		width: 12px;
		height: 12px;
		border: 2px solid var(--border);
		border-top-color: var(--text-primary);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.ocr-text {
		white-space: nowrap;
	}

	.ocr-popover {
		position: absolute;
		top: 100%;
		right: 0;
		margin-top: 4px;
		background: var(--bg-primary);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 10px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
		z-index: 10;
		min-width: 200px;
	}

	.ocr-label {
		display: block;
		font-size: 0.75rem;
		color: var(--text-secondary);
		margin-bottom: 4px;
	}

	.ocr-input {
		width: 100%;
		padding: 5px 8px;
		border: 1px solid var(--border);
		border-radius: 4px;
		font-size: 0.8rem;
		background: var(--bg-primary);
		color: var(--text-primary);
		margin-bottom: 8px;
		box-sizing: border-box;
	}

	.ocr-actions {
		display: flex;
		justify-content: flex-end;
		gap: 6px;
	}

	.ocr-cancel, .ocr-start {
		padding: 4px 10px;
		border-radius: 4px;
		font-size: 0.75rem;
		cursor: pointer;
	}

	.ocr-cancel {
		color: var(--text-secondary);
	}

	.ocr-cancel:hover {
		background: var(--bg-hover);
	}

	.ocr-start {
		background: var(--text-primary);
		color: var(--bg-primary);
	}

	.ocr-start:disabled {
		opacity: 0.3;
		cursor: default;
	}

	.ocr-start:not(:disabled):hover {
		opacity: 0.85;
	}
</style>
