<script lang="ts">
	let {
		reasoning,
		duration = null,
		isStreaming = false
	}: {
		reasoning: string;
		duration: number | null;
		isStreaming: boolean;
	} = $props();

	let expanded = $state(false);

	// Auto-expand while actively thinking, auto-collapse when content starts
	$effect(() => {
		if (isStreaming && duration === null && reasoning) {
			expanded = true;
		} else if (isStreaming && duration !== null) {
			expanded = false;
		}
	});

	function toggle() {
		expanded = !expanded;
	}

	function formatDuration(seconds: number): string {
		if (seconds < 1) return 'Thought for less than a second';
		if (seconds === 1) return 'Thought for 1 second';
		if (seconds < 60) return `Thought for ${seconds} seconds`;
		const mins = Math.floor(seconds / 60);
		const secs = seconds % 60;
		if (secs === 0) return `Thought for ${mins}m`;
		return `Thought for ${mins}m ${secs}s`;
	}

	function getLabel(): string {
		if (isStreaming && duration === null) return 'Thinking';
		if (duration !== null) return formatDuration(duration);
		return 'Reasoning';
	}
</script>

<div class="thinking-block">
	<button class="thinking-toggle" onclick={toggle}>
		<span class="toggle-icon" class:expanded>{@html '&#9656;'}</span>
		<span class="thinking-label">
			{getLabel()}
			{#if isStreaming && duration === null}
				<span class="thinking-dots">
					<span class="dot"></span>
					<span class="dot"></span>
					<span class="dot"></span>
				</span>
			{/if}
		</span>
	</button>

	{#if expanded}
		<div class="thinking-content">
			<div class="thinking-text">{reasoning}</div>
		</div>
	{/if}
</div>

<style>
	.thinking-block {
		margin-bottom: 12px;
		font-size: 0.9375rem;
	}

	.thinking-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 0;
		background: none;
		border: none;
		color: var(--text-secondary);
		cursor: pointer;
		font-size: 0.85rem;
		font-family: inherit;
		transition: color var(--transition);
	}

	.thinking-toggle:hover {
		color: var(--text-primary);
	}

	.toggle-icon {
		display: inline-block;
		font-size: 0.7rem;
		transition: transform 0.2s ease;
		transform: rotate(0deg);
	}

	.toggle-icon.expanded {
		transform: rotate(90deg);
	}

	.thinking-label {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.thinking-dots {
		display: inline-flex;
		gap: 2px;
		margin-left: 2px;
	}

	.dot {
		width: 3px;
		height: 3px;
		border-radius: 50%;
		background: var(--text-secondary);
		animation: thinking-pulse 1.4s ease-in-out infinite;
	}

	.dot:nth-child(2) {
		animation-delay: 0.2s;
	}

	.dot:nth-child(3) {
		animation-delay: 0.4s;
	}

	@keyframes thinking-pulse {
		0%, 80%, 100% {
			opacity: 0.3;
			transform: scale(0.8);
		}
		40% {
			opacity: 1;
			transform: scale(1);
		}
	}

	.thinking-content {
		margin-top: 4px;
		padding: 8px 12px;
		border-left: 2px solid var(--border);
		max-height: 400px;
		overflow-y: auto;
	}

	.thinking-text {
		white-space: pre-wrap;
		color: var(--text-secondary);
		font-size: 0.85rem;
		line-height: 1.5;
	}
</style>
