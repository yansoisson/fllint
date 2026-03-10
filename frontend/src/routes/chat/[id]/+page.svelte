<script lang="ts">
	import { page } from '$app/stores';
	import ChatWindow from '$components/ChatWindow.svelte';
	import InputBar from '$components/InputBar.svelte';
	import {
		selectConversation,
		getMessages,
		getIsStreaming,
		getActiveConversationId
	} from '$lib/stores.svelte';

	// Load conversation when id param changes
	$effect(() => {
		const id = $page.params.id;
		if (id && id !== getActiveConversationId()) {
			selectConversation(id);
		}
	});
</script>

<div class="content" class:centered={getMessages().length === 0 && !getIsStreaming()}>
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

	.content.centered {
		justify-content: center;
	}
</style>
