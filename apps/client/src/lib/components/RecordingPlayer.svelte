<script lang="ts">
	import { Alert, Badge, Button, Spinner, Switch, icons } from '@facile/muse';
	import type { CallId } from '$lib/api';
	import { formatDateTime } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';
	import { PlanLimitError } from '$lib/planLimit';
	import {
		fetchRecordings,
		probeRecording,
		recordingUrl,
		setRecordingShared,
		type Recording
	} from '$lib/meetings';
	import PlanLimitNotice from './PlanLimitNotice.svelte';

	interface Props {
		callId: CallId;
	}

	let { callId }: Props = $props();

	let recordings = $state<Recording[]>([]);
	let loaded = $state(false);
	let error = $state('');
	let planLimit = $state<PlanLimitError | null>(null);
	let busy = $state('');
	let sequence = 0;

	$effect(() => {
		const wanted = callId;
		const ticket = ++sequence;
		recordings = [];
		loaded = false;
		error = '';
		void (async () => {
			try {
				const list = await fetchRecordings(wanted);
				if (ticket === sequence) recordings = list;
			} catch (e) {
				if (ticket === sequence) error = e instanceof Error ? e.message : String(e);
			}
			if (ticket === sequence) loaded = true;
		})();
	});

	async function download(id: string): Promise<void> {
		busy = id;
		error = '';
		try {
			await probeRecording(id);
			const anchor = document.createElement('a');
			anchor.href = recordingUrl(id, true);
			anchor.click();
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			busy = '';
		}
	}

	async function share(recording: Recording, shared: boolean): Promise<void> {
		busy = recording.id;
		error = '';
		planLimit = null;
		try {
			const updated = await setRecordingShared(recording.id, shared);
			recordings = recordings.map((r) => (r.id === updated.id ? updated : r));
		} catch (e) {
			if (e instanceof PlanLimitError) planLimit = e;
			else error = e instanceof Error ? e.message : String(e);
		} finally {
			busy = '';
		}
	}
</script>

<div class="flex flex-col gap-2">
	{#if error}
		<Alert tone="danger" title={t('recordings.failed')}>{error}</Alert>
	{/if}
	{#if planLimit}
		<PlanLimitNotice error={planLimit} />
	{/if}

	{#if !loaded}
		<div class="flex justify-center py-2"><Spinner /></div>
	{:else if recordings.length === 0}
		<p class="text-fc-p text-fc-muted">{t('callPage.noRecording')}</p>
	{:else}
		{#each recordings as recording (recording.id)}
			<div class="flex flex-col gap-1">
				<div class="flex flex-wrap items-center justify-between gap-1">
					<p class="text-fc-p text-fc-ink">{formatDateTime(recording.created_at)}</p>
					{#if recording.shared}
						<Badge tone="info">{t('recordings.shared')}</Badge>
					{:else}
						<Badge>{t('recordings.private')}</Badge>
					{/if}
				</div>
				<video
					controls
					preload="metadata"
					playsinline
					crossorigin="use-credentials"
					src={recordingUrl(recording.id)}
					aria-label={t('recordings.player')}
					class="w-full rounded-fc bg-fc-surface"
				>
					<track kind="captions" />
				</video>
				<div class="flex flex-wrap items-center justify-between gap-1">
					<Button
						variant="outline"
						size="sm"
						icon={icons.download}
						disabled={busy === recording.id}
						onclick={() => download(recording.id)}
					>
						{t('callPage.download')}
					</Button>
					{#if recording.can_share}
						<Switch
							label={t('recordings.share')}
							checked={recording.shared}
							disabled={busy === recording.id}
							onchange={(e) => share(recording, e.currentTarget.checked)}
						/>
					{/if}
				</div>
				{#if recording.can_share}
					<p class="text-fc-subtext text-fc-muted">{t('recordings.shareHint')}</p>
				{/if}
			</div>
		{/each}
	{/if}
</div>
