<script lang="ts">
	import { page } from '$app/state';
	import { Alert, Button, Card, Page, PageHeader, Section, Spinner, icons } from '@facile/muse';
	import { ApiError, fetchCall, fetchCallRecording, type CallDetail } from '$lib/api';
	import { formatDateTime, formatDuration } from '$lib/format';
	import ParticipantTable from '$lib/components/ParticipantTable.svelte';
	import TranscriptPane from '$lib/components/TranscriptPane.svelte';
	import SummaryPanel from '$lib/components/SummaryPanel.svelte';

	const id = $derived(page.params.id ?? '');

	let call = $state<CallDetail | null>(null);
	let loaded = $state(false);
	let error = $state('');
	let denied = $state(false);

	const subtitle = $derived(
		call ? `${formatDateTime(call.started_at)} · ${formatDuration(call.started_at, call.ended_at)}` : ''
	);

	let downloading = $state(false);
	let downloadError = $state('');

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
		downloadError = '';

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

	async function download(callId: CallDetail['id']): Promise<void> {
		downloading = true;
		downloadError = '';
		let objectUrl = '';
		try {
			const { blob, filename } = await fetchCallRecording(callId);
			objectUrl = URL.createObjectURL(blob);
			const anchor = document.createElement('a');
			anchor.href = objectUrl;
			anchor.download = filename;
			anchor.click();
		} catch (e) {
			downloadError = e instanceof Error ? e.message : String(e);
		} finally {
			// Revoking synchronously after click() cancels the download in
			// Chrome; give the browser a turn to pick the blob up first.
			if (objectUrl) setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
			downloading = false;
		}
	}
</script>

<svelte:head><title>Call · Echo</title></svelte:head>

<Page width="lg" gap="section">
	<PageHeader title="Call details" description={subtitle} back={{ href: '/', label: 'Home' }} />

	{#if !loaded}
		<Card><div class="flex justify-center py-4"><Spinner /></div></Card>
	{:else if denied}
		<Alert tone="warning" title="Owner only">
			Only the room owner can see this call. Sign in with the account that created the room.
		</Alert>
	{:else if error}
		<Alert tone="danger" title="Could not load call">{error}</Alert>
	{:else if call}
		<Section title="Participants" card>
			<ParticipantTable participants={call.participants} />
		</Section>

		<Section title="Recording" card>
			{#if call.has_recording}
				{@const recordingId = call.id}
				{#if downloadError}
					<Alert tone="danger" title="Download failed">{downloadError}</Alert>
				{/if}
				<Button
					variant="outline"
					size="sm"
					icon={icons.download}
					disabled={downloading}
					onclick={() => download(recordingId)}
				>
					{downloading ? 'Preparing the download…' : 'Download the video (MP4)'}
				</Button>
			{:else}
				<p class="text-fc-sm text-fc-fg-muted">
					No recording for this call, or the file is not available on this server.
				</p>
			{/if}
		</Section>

		<Section title="AI summary" card>
			<SummaryPanel
				callId={call.id}
				summary={call.summary}
				hasTranscript={Boolean(call.transcript?.trim())}
			/>
		</Section>

		<Section title="Transcript" card>
			<TranscriptPane transcript={call.transcript} />
		</Section>
	{/if}
</Page>
