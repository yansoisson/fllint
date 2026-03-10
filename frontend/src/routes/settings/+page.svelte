<script lang="ts">
	import { openSettings, closeSettings } from '$lib/stores.svelte';

	// Open settings overlay when this route mounts
	$effect(() => {
		openSettings();
		return () => {
			closeSettings();
		};
	});
</script>

<!-- Settings content is rendered by the Settings component in the layout overlay -->
