<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Alert, Button, Card, Page, PageHeader, Section, Spinner, icons } from '@facile/muse';
	import { ApiError, callRecordingUrl, fetchCall, type CallDetail } from '$lib/api';
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

	onMount(async () => {
		try {
			call = await fetchCall(id);
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
			{#if call.recording_path}
				<Button href={callRecordingUrl(call.id)} variant="outline" size="sm" icon={icons.download}>
					Download the video (MP4)
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
