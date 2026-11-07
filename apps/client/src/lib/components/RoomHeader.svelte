<script lang="ts">
	import { Button, Icon } from '@facile/muse';
	import { t } from '$lib/i18n/index.svelte';
	import { callIcons } from '$lib/icons';

	interface Props {
		slug: string;
		historyHref: string;
		count: number;
	}

	let { slug, historyHref, count }: Props = $props();

	let copied = $state(false);

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(window.location.href);
		} catch {
			return;
		}
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}
</script>

<header class="flex items-center justify-between gap-1 px-4 py-3">
	<div class="flex min-w-0 items-center gap-1">
		<a href="/" class="flex items-center gap-1 text-fc-lead text-fc-ink">
			<img src="/logo.svg" alt="" class="size-7" />
			Echo
		</a>
		<span class="truncate text-fc-p text-fc-muted">{slug}</span>
	</div>
	<div class="flex shrink-0 items-center gap-1">
		<span class="flex items-center gap-1 px-2 text-fc-p text-fc-muted" aria-label={t('header.people', { count })}>
			<Icon icon={callIcons.people} size={16} />
			{count}
		</span>
		<Button variant="ghost" size="sm" icon={callIcons.link} onclick={() => void copyLink()}>
			{copied ? t('header.copied') : t('header.copyLink')}
		</Button>
		{#if historyHref}
			<Button variant="ghost" size="sm" href={historyHref}>{t('room.history')}</Button>
		{/if}
	</div>
</header>
