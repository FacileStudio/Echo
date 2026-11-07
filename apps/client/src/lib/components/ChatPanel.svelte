<script lang="ts">
	import gsap from 'gsap';
	import { Button, Icon, Input, TextElevate, prefersReducedMotion } from '@facile/muse';
	import { formatTime } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';
	import { callIcons } from '$lib/icons';

	export interface ChatMessage {
		from: string;
		name: string;
		text: string;
		at: number;
	}

	interface Props {
		messages: ChatMessage[];
		onSend: (text: string) => void;
		open?: boolean;
	}

	let { messages, onSend, open = true }: Props = $props();

	let draft = $state('');
	let listEl: HTMLDivElement | undefined = $state();
	let sendEl: HTMLSpanElement | undefined = $state();

	const ready = $derived(draft.trim().length > 0);

	$effect(() => {
		void messages.length;
		if (listEl)
			listEl.scrollTop = listEl.scrollHeight;
	});

	$effect(() => {
		if (!ready || !sendEl || prefersReducedMotion())
			return;

		gsap.fromTo(
			sendEl,
			{
				opacity: 0.4,
			},
			{
				opacity: 1,
				duration: 0.35,
				ease: 'back.out(2)',
				clearProps: 'opacity,transform',
			}
		);
	});

	function send(e: SubmitEvent) {
		e.preventDefault();
		const text = draft.trim();

		if (!text)
			return;

		onSend(text);
		draft = '';
	}
</script>

<div class="flex min-h-0 flex-1 flex-col gap-1">
	<div class="relative flex min-h-0 flex-1">
		<div bind:this={listEl} class="flex min-h-0 flex-1 bg-fc-surface p-4 rounded-fc-lg flex-col gap-3 overflow-y-auto">
			{#if messages.length === 0}
				<p class="text-fc-sm text-fc-fg-muted">{t('chat.empty')}</p>
			{/if}
			{#each messages as m, i (m.at + '-' + i)}
				<div class="flex flex-col ">
					<p class="flex justify-between items-baseline">
						<TextElevate text={m.name} visible={open} delay={0} class="text-fc-lg text-fc-fg" />
						<span class="text-fc-xs text-fc-fg-muted">{formatTime(m.at)}</span>
					</p>
					<TextElevate text={m.text} visible={open} delay={0.05} class="text-fc-sm wrap-break-word text-fc-fg-muted" />
				</div>
				{/each}
		</div>
	</div>

	<form onsubmit={send} class="flex gap-1">
		<Input bind:value={draft} placeholder={t('chat.placeholder')} aria-label={t('chat.label')} autocomplete="off" />
		<span bind:this={sendEl} class="inline-flex">
			<Button
				type="submit"
				size="lg"
				variant={ready ? 'primary' : 'ghost'}
				class="gap-2.5 {ready ? '' : 'bg-fc-surface disabled:opacity-100'}"
				disabled={!ready}
			>
				<Icon icon={callIcons.send} />{t('chat.send')}
			</Button>
		</span>
	</form>
</div>
