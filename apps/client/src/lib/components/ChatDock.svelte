<script lang="ts">
	import type { Snippet } from 'svelte';
	import gsap from 'gsap';
	import { prefersReducedMotion } from '@facile/muse';
	import ChatPanel, { type ChatMessage } from './ChatPanel.svelte';

	interface Props {
		open: boolean;
		wide: boolean;
		messages: ChatMessage[];
		selfId: string;
		onSend: (text: string) => void;
		onClose: () => void;
		top?: Snippet;
	}

	let { open, wide, messages, selfId, onSend, onClose, top }: Props = $props();

	let asideEl: HTMLElement | undefined = $state();
	let asideShown = false;

	$effect(() => {
		const shown = open;

		if (!asideEl)
			return;

		const instant = !asideShown || prefersReducedMotion();

		asideShown = true;
		gsap.to(asideEl, {
			width: shown ? 'auto' : 0,
			opacity: shown ? 1 : 0,
			x: shown ? 0 : 24,
			duration: instant ? 0 : 0.45,
			ease: 'power3.inOut',
		});
	});
</script>

<aside bind:this={asideEl} class="hidden shrink-0 overflow-hidden md:block" aria-hidden={!open} inert={!open}>
	<div class="flex h-full w-81 flex-col gap-1 pl-1">
		{@render top?.()}
		<ChatPanel {messages} {selfId} {onSend} />
	</div>
</aside>

{#if open && !wide}
	<div class="fixed inset-0 z-40 flex flex-col bg-fc-bg p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] md:hidden">
		<ChatPanel {messages} {selfId} {onSend} {onClose} />
	</div>
{/if}
