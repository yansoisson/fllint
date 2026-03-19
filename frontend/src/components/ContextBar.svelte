<script lang="ts">
	import { getContextUsage, getResponseBuffer } from '$lib/stores.svelte';

	let usage = $derived(getContextUsage());
	let buffer = $derived(getResponseBuffer());

	let totalUsed = $derived(usage ? usage.promptTokens + usage.completionTokens : 0);
	let contextSize = $derived(usage?.contextSize ?? 0);
	let utilization = $derived(contextSize > 0 ? totalUsed / contextSize : 0);
	let isWarning = $derived(contextSize > 0 && utilization >= 0.75);
	let hasContextSize = $derived(contextSize > 0);

	function formatTokens(n: number): string {
		if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
		return String(n);
	}
</script>

{#if totalUsed > 0}
	<div class="context-bar" class:warning={isWarning} title="{totalUsed.toLocaleString()}{hasContextSize ? ' / ' + contextSize.toLocaleString() : ''} tokens used">
		{#if hasContextSize}
			<div class="bar-track">
				<div class="bar-fill" style="width: {Math.min(utilization * 100, 100)}%"></div>
			</div>
			<span class="context-label">{formatTokens(totalUsed)} / {formatTokens(contextSize)}</span>
		{:else}
			<span class="context-label">{formatTokens(totalUsed)} tokens</span>
		{/if}
	</div>
{/if}

<style>
	.context-bar {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-left: 10px;
		flex-shrink: 0;
	}

	.bar-track {
		width: 48px;
		height: 3px;
		border-radius: 2px;
		background: var(--bg-tertiary);
		overflow: hidden;
	}

	.bar-fill {
		height: 100%;
		border-radius: 2px;
		background: var(--text-muted);
		transition: width 0.4s ease;
	}

	.warning .bar-fill {
		background: var(--warning-color, #d97706);
	}

	.context-label {
		font-size: 0.7rem;
		color: var(--text-muted);
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
	}

	.warning .context-label {
		color: var(--warning-color, #d97706);
	}
</style>
