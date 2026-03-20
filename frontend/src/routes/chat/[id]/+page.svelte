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

	// Load conversation when id param changes.
	// Track last loaded ID locally to avoid reacting to external
	// activeConversationId changes (e.g. navigateToNewConversation).
	let lastLoadedId = '';
	$effect(() => {
		const id = $page.params.id;
		if (id && id !== lastLoadedId) {
			lastLoadedId = id;
			selectConversation(id);
		}
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

	.center-group.active {
		display: flex;
		flex-direction: column;
		margin: auto 0;
	}
</style>
