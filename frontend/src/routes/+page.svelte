<script lang="ts">
	import ChatWindow from '$components/ChatWindow.svelte';
	import InputBar from '$components/InputBar.svelte';
	import { getMessages, getIsStreaming, newConversation } from '$lib/stores.svelte';

	// Ensure we're in "new chat" state when visiting /
	$effect(() => {
		newConversation();
	});

	let empty = $derived(getMessages().length === 0 && !getIsStreaming());
</script>

<div class="content" class:centered={empty}>
	<div class="center-group" class:active={empty}>
		<ChatWindow />
		<InputBar />
	</div>
</div>

<style>
	.content {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.center-group {
		display: contents;
	}

	/* When empty, the group centers itself vertically via margin auto */
	.center-group.active {
		display: flex;
		flex-direction: column;
		margin: auto 0;
	}
</style>
