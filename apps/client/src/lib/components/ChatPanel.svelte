<script module lang="ts">
	export interface ChatMessage {
		from: string;
		name: string;
		text: string;
		at: number;
	}

	export interface ChatGroup {
		from: string;
		name: string;
		at: number;
		messages: ChatMessage[];
	}

	const GROUP_GAP_MS = 5 * 60 * 1000;

	/** Folds consecutive messages from one sender, sent within five minutes, under one header. */
	export function groupChat(messages: ChatMessage[]): ChatGroup[] {
		const groups: ChatGroup[] = [];
		for (const m of messages) {
			const last = groups.at(-1);
			const tail = last?.messages.at(-1);
			if (last && tail && last.from === m.from && m.at - tail.at < GROUP_GAP_MS) last.messages.push(m);
			else groups.push({ from: m.from, name: m.name, at: m.at, messages: [m] });
		}
		return groups;
	}
</script>

<script lang="ts">
	import gsap from 'gsap';
	import { Button, Icon, IconButton, Input, prefersReducedMotion } from '@facile/muse';
	import { formatTime } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';
	import { callIcons } from '$lib/icons';

	interface Props {
		messages: ChatMessage[];
		selfId: string;
		onSend: (text: string) => void;
		onClose?: () => void;
	}

	let { messages, selfId, onSend, onClose }: Props = $props();

	let draft = $state('');
	let listEl: HTMLDivElement | undefined = $state();
	let sendEl: HTMLSpanElement | undefined = $state();

	const ready = $derived(draft.trim().length > 0);
	const groups = $derived(groupChat(messages));

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

<section class="flex min-h-0 flex-1 flex-col gap-1" aria-label={t('chat.title')}>
	{#if onClose}
		<header class="flex items-center justify-between px-1 py-1">
			<h2 class="text-fc-lead text-fc-ink">{t('chat.title')}</h2>
			<IconButton variant="ghost" aria-label={t('chat.close')} onclick={onClose}><Icon icon={callIcons.close} /></IconButton>
		</header>
	{/if}

	<div
		bind:this={listEl}
		role="log"
		aria-live="polite"
		aria-label={t('chat.title')}
		class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto rounded-fc bg-fc-surface p-3"
	>
		{#if messages.length === 0}
			<p class="text-fc-p text-fc-muted">{t('chat.empty')}</p>
		{/if}
		{#each groups as g (g.from + '-' + g.at)}
			{@const mine = g.from === selfId}
			<article class="flex flex-col gap-1 {mine ? 'items-end' : 'items-start'}">
				<header class="flex items-baseline gap-2 px-1 {mine ? 'flex-row-reverse' : ''}">
					<span class="text-fc-subtext font-semibold {mine ? 'text-fc-brand' : 'text-fc-ink'}">{mine ? t('chat.you') : g.name}</span>
					<time class="text-fc-subtext text-fc-muted" datetime={new Date(g.at).toISOString()}>{formatTime(g.at)}</time>
				</header>
				{#each g.messages as m, i (m.at + '-' + i)}
					<p
						class="max-w-[85%] rounded-fc px-3 py-2 text-fc-p wrap-break-word whitespace-pre-wrap text-fc-ink {mine
							? 'bg-fc-brand-bg'
							: 'bg-fc-bg'}"
					>
						{m.text}
					</p>
				{/each}
			</article>
		{/each}
	</div>

	<form onsubmit={send} class="flex gap-1">
		<Input bind:value={draft} placeholder={t('chat.placeholder')} aria-label={t('chat.label')} autocomplete="off" class="text-fc-p" />
		<span bind:this={sendEl} class="inline-flex">
			<Button
				type="submit"
				size="lg"
				variant={ready ? 'primary' : 'ghost'}
				class="gap-2.5 {ready ? '' : 'bg-fc-surface disabled:opacity-100'}"
				disabled={!ready}
			>
				<Icon icon={callIcons.send} /><span class="sr-only sm:not-sr-only">{t('chat.send')}</span>
			</Button>
		</span>
	</form>
</section>
