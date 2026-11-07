<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { Alert, Card, Page, PageHeader, Section, Spinner } from '@facile/muse';
	import { ApiError, fetchCall, type CallDetail } from '$lib/api';
	import { formatDateTime, formatDuration } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';
	import { loadSession } from '$lib/session.svelte';
	import ParticipantTable from '$lib/components/ParticipantTable.svelte';
	import RecordingPlayer from '$lib/components/RecordingPlayer.svelte';
	import TranscriptPane from '$lib/components/TranscriptPane.svelte';
	import SummaryPanel from '$lib/components/SummaryPanel.svelte';

	const id = $derived(page.params.id ?? '');

	onMount(() => {
		void loadSession();
	});

	let call = $state<CallDetail | null>(null);
	let loaded = $state(false);
	let error = $state('');
	let denied = $state(false);

	const subtitle = $derived(
		call ? `${formatDateTime(call.started_at)} · ${formatDuration(call.started_at, call.ended_at)}` : ''
	);

	// SvelteKit reuses this component between two /calls/[id] routes, so the
	// fetch is keyed on the id rather than on mount. `sequence` drops a slow
	// answer for a previous id that lands after a newer one.
	let sequence = 0;

	$effect(() => {
		const wanted = id;
		const ticket = ++sequence;
		call = null;
		loaded = false;
		error = '';
		denied = false;

		if (!wanted) {
			loaded = true;
			return;
		}

		void (async () => {
			try {
				const detail = await fetchCall(wanted);
				if (ticket !== sequence) return;
				call = detail;
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

<svelte:head><title>{t('title.call')}</title></svelte:head>

<Page width="lg" class="gap-1 px-4 py-10">
	<PageHeader title={t('callPage.title')} description={subtitle} back={{ href: call ? '/history' : '/', label: call ? t('meetings.title') : t('callPage.home') }} />

	{#if !loaded}
		<Card><div class="flex justify-center py-4"><Spinner /></div></Card>
	{:else if denied}
		<Alert tone="warning" title={t('historyPage.ownerOnly')}>{t('callPage.ownerOnlyBody')}</Alert>
	{:else if error}
		<Alert tone="danger" title={t('callPage.loadFailed')}>{error}</Alert>
	{:else if call}
		<Section title={t('callPage.participants')} card>
			<ParticipantTable participants={call.participants} />
		</Section>

		<Section title={t('callPage.recording')} card>
			<RecordingPlayer callId={call.id} />
		</Section>

		<Section title={t('callPage.summary')} card>
			<SummaryPanel
				callId={call.id}
				summary={call.summary}
				hasTranscript={call.has_transcript}
				canGenerate={call.role === 'owner'}
			/>
		</Section>

		<Section title={t('callPage.transcript')} card>
			<TranscriptPane callId={call.id} hasTranscript={call.has_transcript} />
		</Section>
	{/if}
</Page>
