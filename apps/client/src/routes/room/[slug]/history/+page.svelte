<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { Alert, Button, Card, EmptyState, Page, PageHeader, Spinner, icons } from '@facile/muse';
	import { ApiError, fetchRoomCalls, type CallListItem } from '$lib/api';
	import CallHistoryTable from '$lib/components/CallHistoryTable.svelte';
	import { t } from '$lib/i18n/index.svelte';
	import { loadSession } from '$lib/session.svelte';

	const slug = $derived(page.params.slug ?? '');

	onMount(() => {
		void loadSession();
	});

	let calls = $state<CallListItem[]>([]);
	let loaded = $state(false);
	let error = $state('');
	let denied = $state(false);

	function newestFirst(list: CallListItem[]): CallListItem[] {
		return [...list].sort(
			(a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
		);
	}

	// SvelteKit reuses this component between two /room/[slug]/history routes,
	// so the fetch is keyed on the slug rather than on mount. `sequence` drops a
	// slow answer for a previous slug that lands after a newer one.
	let sequence = 0;

	$effect(() => {
		const wanted = slug;
		const ticket = ++sequence;
		calls = [];
		loaded = false;
		error = '';
		denied = false;

		if (!wanted) {
			loaded = true;
			return;
		}

		void (async () => {
			try {
				const list = await fetchRoomCalls(wanted);
				if (ticket !== sequence) return;
				calls = newestFirst(list);
			} catch (e) {
				if (ticket !== sequence) return;
				if (e instanceof ApiError && (e.status === 401 || e.status === 403)) {
					denied = true;
				} else {
					error = e instanceof Error ? e.message : String(e);
				}
			}
			if (ticket === sequence) loaded = true;
		})();
	});
</script>

<svelte:head><title>{t('title.history', { slug })}</title></svelte:head>

<Page width="lg" class="gap-1 px-4 py-10">
	<PageHeader
		title={t('historyPage.title', { slug })}
		description={t('historyPage.description')}
		back={{ href: `/room/${slug}`, label: t('historyPage.back') }}
	/>

	{#if !loaded}
		<Card><div class="flex justify-center py-4"><Spinner /></div></Card>
	{:else if denied}
		<Alert tone="warning" title={t('historyPage.ownerOnly')}>{t('historyPage.ownerOnlyBody', { slug })}</Alert>
		<div><Button href="/login" variant="outline" size="sm">{t('login.signIn')}</Button></div>
	{:else if error}
		<Alert tone="danger" title={t('historyPage.loadFailed')}>{error}</Alert>
	{:else if calls.length === 0}
		<EmptyState
			icon={icons.history}
			title={t('historyPage.emptyTitle')}
			description={t('historyPage.emptyBody')}
		>
			<Button href="/room/{slug}" size="sm">{t('historyPage.openRoom')}</Button>
		</EmptyState>
	{:else}
		<CallHistoryTable {calls} />
	{/if}
</Page>
