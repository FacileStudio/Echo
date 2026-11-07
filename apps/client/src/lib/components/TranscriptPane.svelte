<script lang="ts">
	import { Alert, Button, Input, Spinner, icons } from '@facile/muse';
	import type { CallId } from '$lib/api';
	import PlanLimitNotice from './PlanLimitNotice.svelte';
	import { PlanLimitError } from '$lib/planLimit';
	import { t } from '$lib/i18n/index.svelte';
	import { downloadTranscript, fetchTranscript, type Transcript, type TranscriptFormat } from '$lib/transcript';

	interface Props {
		callId: CallId;
		hasTranscript: boolean;
	}

	let { callId, hasTranscript }: Props = $props();

	let search = $state('');
	let transcript = $state<Transcript | null>(null);
	let loading = $state(false);
	let error = $state('');
	let planLimit = $state<PlanLimitError | null>(null);
	let sequence = 0;

	function clock(ms: number | undefined): string {
		if (ms === undefined) return '';
		const total = Math.floor(ms / 1000);
		const parts = [Math.floor(total / 3600), Math.floor((total % 3600) / 60), total % 60];
		return `[${parts.map((n) => String(n).padStart(2, '0')).join(':')}] `;
	}

	$effect(() => {
		const wanted = search;
		const id = callId;
		const ticket = ++sequence;
		if (!hasTranscript) return;
		loading = true;
		const timer = setTimeout(async () => {
			try {
				const result = await fetchTranscript(id, wanted);
				if (ticket !== sequence) return;
				transcript = result;
				error = '';
				planLimit = null;
			} catch (e) {
				if (ticket === sequence) fail(e);
			}
			if (ticket === sequence) loading = false;
		}, wanted ? 250 : 0);
		return () => clearTimeout(timer);
	});

	function fail(e: unknown): void {
		if (e instanceof PlanLimitError) planLimit = e;
		else error = e instanceof Error ? e.message : String(e);
	}

	async function save(format: TranscriptFormat): Promise<void> {
		error = '';
		try {
			await downloadTranscript(callId, format);
		} catch (e) {
			fail(e);
		}
	}
</script>

{#if !hasTranscript}
	<p class="text-fc-p text-fc-muted">{t('transcript.none')}</p>
{:else if planLimit}
	<PlanLimitNotice error={planLimit} />
{:else}
	<div class="flex flex-col gap-1">
		<div class="flex flex-wrap items-center gap-1">
			<Input
				type="search"
				bind:value={search}
				placeholder={t('transcript.search')}
				aria-label={t('transcript.search')}
				class="text-fc-p"
				autocomplete="off"
			/>
			<Button variant="outline" size="sm" icon={icons.download} onclick={() => save('txt')}>
				{t('transcript.downloadTxt')}
			</Button>
			<Button variant="outline" size="sm" icon={icons.download} onclick={() => save('vtt')}>
				{t('transcript.downloadVtt')}
			</Button>
		</div>

		{#if error}
			<Alert tone="danger" title={t('transcript.failed')}>{error}</Alert>
		{/if}

		{#if !transcript}
			<div class="flex justify-center py-2"><Spinner /></div>
		{:else if transcript.lines.length === 0}
			<p class="text-fc-p text-fc-muted">{t('transcript.noMatch')}</p>
		{:else}
			{#if search.trim()}
				<p class="text-fc-subtext text-fc-muted" aria-live="polite">
					{t('transcript.matches', { count: transcript.lines.length, total: transcript.total })}
				</p>
			{/if}
			<ul
				class="max-h-96 list-none overflow-y-auto rounded-fc bg-fc-surface p-4 text-fc-p text-fc-ink"
				aria-busy={loading}
			>
				{#each transcript.lines as line, index (index)}
					<li class="whitespace-pre-wrap">
						<span class="text-fc-muted">{clock(line.offset_ms)}</span>
						<span class="font-medium">{line.speaker || t('transcript.unknownSpeaker')}</span>: {line.text}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
{/if}
