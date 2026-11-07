<script lang="ts">
	import { Alert, Button, Spinner, icons } from '@facile/muse';
	import { ApiError, generateCallSummary, type CallId, type CallSummary } from '$lib/api';
	import { formatDateTime } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		callId: CallId;
		summary?: CallSummary;
		hasTranscript: boolean;
	}

	let { callId, summary, hasTranscript }: Props = $props();

	let generated = $state<CallSummary | undefined>(undefined);
	let pending = $state(false);

	const current = $derived(generated ?? summary);

	let error = $state('');

	function messageFor(e: unknown): string {
		if (e instanceof ApiError && e.status === 503) {
			return t('summary.notConfigured');
		}
		if (e instanceof ApiError && e.status === 409) {
			return t('summary.noTranscript');
		}
		if (e instanceof ApiError && e.status === 403) {
			return t('summary.ownerOnly');
		}
		return e instanceof Error ? e.message : String(e);
	}

	async function generate() {
		pending = true;
		error = '';
		try {
			generated = await generateCallSummary(callId);
		} catch (e) {
			error = messageFor(e);
		} finally {
			pending = false;
		}
	}
</script>

<div class="flex flex-col gap-1">
	{#if error}
		<Alert tone="danger" title={t('summary.unavailable')}>{error}</Alert>
	{/if}

	{#if pending}
		<div class="flex items-center gap-1">
			<Spinner />
			<p class="text-fc-p text-fc-muted">{t('summary.generating')}</p>
		</div>
	{:else if current}
		<p class="text-fc-p whitespace-pre-wrap text-fc-ink">{current.content}</p>
		<p class="text-fc-subtext text-fc-muted">
			{t('summary.generatedBy', { model: current.model, date: formatDateTime(current.created_at) })}
		</p>
	{:else if !hasTranscript}
		<p class="text-fc-p text-fc-muted">{t('summary.noneEncrypted')}</p>
	{:else}
		<p class="text-fc-p text-fc-muted">{t('summary.none')}</p>
	{/if}

	{#if hasTranscript}
		<div>
			<Button
				variant={current ? 'outline' : 'primary'}
				size="sm"
				icon={current ? icons.refresh : icons.bolt}
				disabled={pending}
				onclick={generate}
			>
				{current ? t('summary.regenerate') : t('summary.generate')}
			</Button>
		</div>
	{/if}
</div>
