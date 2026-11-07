<script lang="ts">
	import { onMount } from 'svelte';
	import { Alert, Button, Card, EmptyState, Page, PageHeader, Spinner, icons } from '@facile/muse';
	import { ApiError } from '$lib/api';
	import CallHistoryTable from '$lib/components/CallHistoryTable.svelte';
	import { t } from '$lib/i18n/index.svelte';
	import { fetchMeetings, type MeetingsPage } from '$lib/meetings';
	import { loadSession } from '$lib/session.svelte';

	let current = $state(1);
	let result = $state<MeetingsPage | null>(null);
	let loaded = $state(false);
	let error = $state('');
	let denied = $state(false);
	let sequence = 0;

	const pages = $derived(result ? Math.max(1, Math.ceil(result.total / result.page_size)) : 1);

	onMount(() => {
		void loadSession();
	});

	$effect(() => {
		const wanted = current;
		const ticket = ++sequence;
		loaded = false;
		error = '';
		void (async () => {
			try {
				const next = await fetchMeetings(wanted);
				if (ticket === sequence) result = next;
			} catch (e) {
				if (ticket !== sequence) return;
				if (e instanceof ApiError && e.status === 401) denied = true;
				else error = e instanceof Error ? e.message : String(e);
			}
			if (ticket === sequence) loaded = true;
		})();
	});
</script>

<svelte:head><title>{t('title.meetings')}</title></svelte:head>

<Page width="lg" class="gap-1 px-4 py-10">
	<PageHeader
		title={t('meetings.title')}
		description={t('meetings.description')}
		back={{ href: '/', label: t('callPage.home') }}
	/>

	{#if !loaded && !result}
		<Card><div class="flex justify-center py-4"><Spinner /></div></Card>
	{:else if denied}
		<Alert tone="warning" title={t('meetings.signInTitle')}>{t('meetings.signInBody')}</Alert>
		<div><Button href="/login" variant="outline" size="sm">{t('login.signIn')}</Button></div>
	{:else if error}
		<Alert tone="danger" title={t('historyPage.loadFailed')}>{error}</Alert>
	{:else if result && result.items.length === 0 && current === 1}
		<EmptyState icon={icons.history} title={t('meetings.emptyTitle')} description={t('meetings.emptyBody')}>
			<Button href="/" size="sm">{t('historyPage.openRoom')}</Button>
		</EmptyState>
	{:else if result}
		<CallHistoryTable calls={result.items} showMeeting />
		<div class="flex items-center justify-between gap-1">
			<Button variant="outline" size="sm" disabled={current <= 1 || !loaded} onclick={() => (current -= 1)}>
				{t('meetings.previous')}
			</Button>
			<p class="text-fc-subtext text-fc-muted" aria-live="polite">
				{t('meetings.pageOf', { page: result.page, pages })}
			</p>
			<Button variant="outline" size="sm" disabled={current >= pages || !loaded} onclick={() => (current += 1)}>
				{t('meetings.next')}
			</Button>
		</div>
	{/if}
</Page>
