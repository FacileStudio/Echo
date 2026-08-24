<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Alert, Button, Card, EmptyState, Page, PageHeader, Spinner, icons } from '@facile/muse';
	import { ApiError, fetchRoomCalls, type CallListItem } from '$lib/api';
	import CallHistoryTable from '$lib/components/CallHistoryTable.svelte';

	const slug = $derived(page.params.slug ?? '');

	let calls = $state<CallListItem[]>([]);
	let loaded = $state(false);
	let error = $state('');
	let denied = $state(false);

	function newestFirst(list: CallListItem[]): CallListItem[] {
		return [...list].sort(
			(a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
		);
	}

	onMount(async () => {
		try {
			calls = newestFirst(await fetchRoomCalls(slug));
		} catch (e) {
			if (e instanceof ApiError && (e.status === 401 || e.status === 403)) {
				denied = true;
			} else {
				error = e instanceof Error ? e.message : String(e);
			}
		}
		loaded = true;
	});
</script>

<svelte:head><title>History · {slug} · Echo</title></svelte:head>

<Page width="lg" gap="content">
	<PageHeader
		title="History of “{slug}”"
		description="Past calls in this room, newest first."
		back={{ href: `/room/${slug}`, label: 'Back to the room' }}
	/>

	{#if !loaded}
		<Card><div class="flex justify-center py-4"><Spinner /></div></Card>
	{:else if denied}
		<Alert tone="warning" title="Owner only">
			Only the room owner can see this history. Sign in with the account that created “{slug}”.
		</Alert>
		<div><Button href="/login" variant="outline" size="sm">Sign in</Button></div>
	{:else if error}
		<Alert tone="danger" title="Could not load history">{error}</Alert>
	{:else if calls.length === 0}
		<EmptyState
			icon={icons.history}
			title="No calls yet"
			description="Once a call has taken place in this room, it will show up here."
		>
			<Button href="/room/{slug}" size="sm">Open the room</Button>
		</EmptyState>
	{:else}
		<CallHistoryTable {calls} />
	{/if}
</Page>
