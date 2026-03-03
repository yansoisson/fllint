<script lang="ts">
	import { getPendingImage, setPendingImage } from '$lib/stores.svelte';
</script>

{#if getPendingImage()}
	<div class="preview">
		<img src={getPendingImage()?.preview} alt="Upload preview" />
		<button class="remove" onclick={() => setPendingImage(null)}>&times;</button>
	</div>
{/if}

<style>
	.preview {
		position: relative;
		display: inline-block;
		margin: 4px 8px 4px 12px;
	}

	img {
		max-height: 100px;
		max-width: 180px;
		border-radius: 12px;
		border: 1px solid var(--border);
		object-fit: cover;
	}

	.remove {
		position: absolute;
		top: -6px;
		right: -6px;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: var(--text-secondary);
		color: white;
		font-size: 12px;
		display: flex;
		align-items: center;
		justify-content: center;
		line-height: 1;
		transition: background var(--transition);
	}

	.remove:hover {
		background: var(--text-primary);
	}
</style>
