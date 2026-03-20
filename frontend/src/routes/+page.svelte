<script lang="ts">
	import ChatWindow from '$components/ChatWindow.svelte';
	import InputBar from '$components/InputBar.svelte';
	import { newConversation } from '$lib/stores.svelte';

	// Ensure we're in "new chat" state when visiting /
	$effect(() => {
		newConversation();
	});
</script>

<div class="content">
	<ChatWindow />
	<InputBar />
</div>

<style>
	.content {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}
</style>
