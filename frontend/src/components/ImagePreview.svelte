<script lang="ts">
	import { getPendingImages, removePendingImage } from '$lib/stores.svelte';
</script>

{#if getPendingImages().length > 0}
	<div class="previews">
		{#each getPendingImages() as img, i}
			<div class="preview">
				<img src={img.preview} alt="Upload preview" />
				<button class="remove" onclick={() => removePendingImage(i)}>&times;</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.previews {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		margin: 4px 8px 4px 12px;
	}

	.preview {
		position: relative;
		display: inline-block;
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
