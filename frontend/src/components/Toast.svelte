<script lang="ts">
	import { getNotification, dismissNotification } from '$lib/stores.svelte';
</script>

{#if getNotification()}
	<div class="toast" class:error={getNotification()?.type === 'error'}>
		<span>{getNotification()?.message}</span>
		<button class="dismiss" onclick={dismissNotification} aria-label="Dismiss">&times;</button>
	</div>
{/if}

<style>
	.toast {
		position: fixed;
		top: 16px;
		right: 16px;
		z-index: 100;
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px 16px;
		border-radius: var(--radius);
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		box-shadow: var(--shadow-lg);
		font-size: 0.875rem;
		color: var(--text-primary);
		max-width: 400px;
		animation: slideIn 0.2s ease-out;
	}

	.toast.error {
		background: var(--error-bg, #fef2f2);
		border-color: var(--error-border, #fecaca);
		color: var(--error-text, #991b1b);
	}

	.dismiss {
		color: inherit;
		opacity: 0.6;
		font-size: 18px;
		padding: 2px 6px;
		border-radius: 4px;
		cursor: pointer;
		flex-shrink: 0;
	}

	.dismiss:hover {
		opacity: 1;
	}

	@keyframes slideIn {
		from {
			transform: translateY(-8px);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
	}
</style>
