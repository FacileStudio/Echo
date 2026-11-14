<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { Alert, Badge, Button, Card, EmptyState, Page, PageHeader, Spinner, icons } from '@facile/muse';
	import { ApiError } from '$lib/api';
	import { formatDateTime } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';
	import { answerPending, clientName, fetchPending, type PendingAction } from '$lib/pending';
	import { isOwnSessionRefusal } from '$lib/refusal';
	import { loadSession } from '$lib/session.svelte';

	let actions = $state<PendingAction[]>([]);
	let loaded = $state(false);
	let denied = $state(false);
	let notOwnSession = $state(false);
	let error = $state('');
	let notice = $state('');
	let busy = $state('');

	const focused = $derived(page.url.searchParams.get('action') ?? '');
	const ordered = $derived([
		...actions.filter((action) => action.id === focused),
		...actions.filter((action) => action.id !== focused)
	]);

	onMount(() => {
		void loadSession();
		void load();
	});

	/** Loads the open actions. */
	async function load() {
		error = '';
		try {
			actions = await fetchPending();
		} catch (e) {
			if (e instanceof ApiError && e.status === 401) denied = true;
			else if (isOwnSessionRefusal(e)) notOwnSession = true;
			else error = e instanceof Error ? e.message : String(e);
		}
		loaded = true;
	}

	/** Confirms or dismisses one action, then drops it from the list. */
	async function answer(action: PendingAction, choice: 'confirm' | 'dismiss') {
		busy = action.id;
		notice = '';
		try {
			await answerPending(action.id, choice);
			actions = actions.filter((item) => item.id !== action.id);
			notice = choice === 'confirm' ? t('pending.deleted') : t('pending.dismissed');
		} catch (e) {
			if (isOwnSessionRefusal(e)) notOwnSession = true;
			else {
				error = e instanceof Error ? e.message : String(e);
				await load();
			}
		}
		busy = '';
	}
</script>

<svelte:head><title>{t('title.pending')}</title></svelte:head>

<Page width="md" class="gap-1 px-4 py-10">
	<PageHeader title={t('pending.title')} description={t('pending.description')} back={{ href: '/', label: t('callPage.home') }} />

	{#if notice}
		<Alert tone="success">{notice}</Alert>
	{/if}

	{#if !loaded}
		<Card><div class="flex justify-center py-4"><Spinner /></div></Card>
	{:else if denied}
		<Alert tone="warning" title={t('pending.signInTitle')}>{t('pending.signInBody')}</Alert>
		<div><Button href="/login" variant="outline" size="sm">{t('login.signIn')}</Button></div>
	{:else if notOwnSession}
		<Alert tone="warning" title={t('pending.ownSessionTitle')}>{t('pending.ownSessionBody')}</Alert>
		<div><Button href="/login" variant="outline" size="sm">{t('login.signIn')}</Button></div>
	{:else}
		{#if error}
			<Alert tone="danger" title={t('pending.loadFailed')}>{error}</Alert>
		{/if}
		{#if actions.length === 0 && !error}
			<EmptyState icon={icons.check} title={t('pending.emptyTitle')} description={t('pending.emptyBody')} />
		{/if}
		{#each ordered as action (action.id)}
			<Card class="flex flex-col gap-3">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<Badge tone="danger" class="shrink-0 whitespace-nowrap">{t('pending.deleteRoom')}</Badge>
					<span class="text-fc-subtext text-fc-muted">
						{t('pending.askedBy', { client: clientName(action.client_id), when: formatDateTime(action.created_at) })}
					</span>
				</div>
				{#if action.room}
					<div class="flex flex-wrap items-center justify-between gap-2">
						<p class="min-w-0 break-words font-medium">{action.room.name} · /room/{action.room.slug}</p>
						<Badge class="shrink-0 whitespace-nowrap">{t('pending.meetings', { count: action.room.meetings })}</Badge>
					</div>
					<p class="text-fc-subtext text-fc-muted">{t('pending.deleteNote')}</p>
				{:else}
					<p class="break-words text-fc-subtext text-fc-muted">{action.summary}</p>
				{/if}
				{#if action.state === 'changed'}
					<Alert tone="warning">{t('pending.changed')}</Alert>
				{:else if action.state === 'missing'}
					<Alert tone="warning">{t('pending.missing')}</Alert>
				{/if}
				<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
					<Button variant="ghost" class="w-full sm:w-auto" disabled={busy !== ''} onclick={() => answer(action, 'dismiss')}>
						{t('pending.dismiss')}
					</Button>
					{#if action.state === 'ready'}
						<Button variant="danger" class="w-full sm:w-auto" icon={icons.remove} disabled={busy !== ''} onclick={() => answer(action, 'confirm')}>
							{t('pending.confirm')}
						</Button>
					{/if}
				</div>
			</Card>
		{/each}
	{/if}
</Page>
