<script lang="ts">
	import { Button, Input } from '@facile/muse';

	export interface ChatMessage {
		from: string;
		name: string;
		text: string;
		at: number;
	}

	interface Props {
		messages: ChatMessage[];
		onSend: (text: string) => void;
	}

	let { messages, onSend }: Props = $props();

	let draft = $state('');
	let listEl: HTMLDivElement | undefined = $state();

	$effect(() => {
		messages.length;
		if (listEl) listEl.scrollTop = listEl.scrollHeight;
	});

	function send(e: SubmitEvent) {
		e.preventDefault();
		const text = draft.trim();
		if (!text) return;
		onSend(text);
		draft = '';
	}
</script>

<div class="flex h-full min-h-0 flex-col gap-3">
	<div bind:this={listEl} class="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
		{#if messages.length === 0}
			<p class="text-fc-sm text-fc-fg-muted">No messages yet. Chat is visible only in this call.</p>
		{/if}
		{#each messages as m, i ((m.at ?? '') + '-' + i)}
			<div class="text-fc-sm">
				<span class="font-medium text-fc-fg">{m.name}</span>
				<span class="ml-2 text-fc-xs text-fc-fg-muted">
					{new Date(m.at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
				</span>
				<p class="text-fc-fg">{m.text}</p>
			</div>
		{/each}
	</div>

	<form onsubmit={send} class="flex gap-2">
		<Input bind:value={draft} placeholder="Message…" aria-label="Chat message" />
		<Button type="submit" size="md">Send</Button>
	</form>
</div>
