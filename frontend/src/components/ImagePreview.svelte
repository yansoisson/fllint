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
		margin: 8px 0;
	}

	img {
		max-height: 120px;
		max-width: 200px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		object-fit: cover;
	}

	.remove {
		position: absolute;
		top: -8px;
		right: -8px;
		width: 22px;
		height: 22px;
		border-radius: 50%;
		background: var(--accent);
		color: white;
		font-size: 14px;
		display: flex;
		align-items: center;
		justify-content: center;
		line-height: 1;
	}

	.remove:hover {
		background: var(--accent-hover);
	}
</style>
