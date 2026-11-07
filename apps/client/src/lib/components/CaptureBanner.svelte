<script lang="ts">
	import { Alert, Badge, Button, Select, Spinner } from '@facile/muse';
	import { pendingQuestions } from '$lib/capture';
	import type { CaptureSession } from '$lib/captureSession.svelte';
	import { t } from '$lib/i18n/index.svelte';
	import { speechLanguages, type SpeechLanguage } from '$lib/stt';

	interface Props {
		session: CaptureSession;
	}

	let { session }: Props = $props();

	const state = $derived(session.state!);
	const phase = $derived(session.phase);
	const progress = $derived(session.progress);
	const language = $derived(session.language);
	const leftOut = $derived(session.left);
	const onAnswer = (answer: { transcribe?: boolean; record?: boolean }) => void session.answer(answer);
	const onLanguage = (code: SpeechLanguage) => session.chooseLanguage(code);

	const ask = $derived(pendingQuestions(state));
	const asking = $derived(ask.transcribe || ask.record);
	const names: Record<SpeechLanguage, string> = { fr: 'Français', en: 'English' };

	function answer(yes: boolean) {
		onAnswer({
			...(ask.transcribe ? { transcribe: yes } : {}),
			...(ask.record ? { record: yes } : {})
		});
	}

	function withdraw() {
		onAnswer({
			...(state.transcription && state.you.transcribe === true ? { transcribe: false } : {}),
			...(state.recording && state.you.record === true ? { record: false } : {})
		});
	}

	const included = $derived(
		(!state.transcription || state.you.transcribe === true) && (!state.recording || state.you.record === true)
	);
</script>

<div class="flex flex-col gap-1 px-4 pb-3" role="status">
	{#if asking}
		<Alert tone="warning" title={t('capture.askTitle')}>
			<div class="flex flex-col gap-1">
				{#if ask.transcribe}<p>{t('capture.askTranscribe')}</p>{/if}
				{#if ask.record}<p>{t('capture.askRecord')}</p>{/if}
				{#if ask.transcribe}
					<Select
						value={language}
						aria-label={t('capture.language')}
						onchange={(e) => onLanguage(e.currentTarget.value as SpeechLanguage)}
					>
						{#each speechLanguages as code (code)}
							<option value={code}>{names[code]}</option>
						{/each}
					</Select>
				{/if}
				<div class="flex gap-1">
					<Button size="sm" onclick={() => answer(true)}>{t('capture.accept')}</Button>
					<Button size="sm" variant="outline" onclick={() => answer(false)}>{t('capture.decline')}</Button>
				</div>
			</div>
		</Alert>
	{/if}

	<div class="flex flex-wrap items-center justify-center gap-1">
		{#if state.transcription}<Badge tone="warning">{t('capture.transcribing')}</Badge>{/if}
		{#if state.recording}<Badge tone="danger">{t('capture.recording')}</Badge>{/if}
		{#if !asking}
			<Badge tone={included ? 'success' : 'neutral'}>{included ? t('capture.youIn') : t('capture.youOut')}</Badge>
		{/if}
		{#if state.owner && leftOut > 0}
			<Badge tone="neutral">{t('capture.leftOut', { count: leftOut })}</Badge>
		{/if}
		{#if !asking && (state.you.transcribe === true || state.you.record === true)}
			<Button size="sm" variant="ghost" onclick={withdraw}>{t('capture.withdraw')}</Button>
		{/if}
	</div>

	{#if state.you.transcribe === true && state.transcription}
		{#if phase === 'downloading'}
			<p class="flex items-center justify-center gap-1 text-fc-subtext text-fc-muted">
				<Spinner />{t('capture.downloading', { percent: Math.round(progress * 100) })}
			</p>
		{:else if phase === 'starting'}
			<p class="flex items-center justify-center gap-1 text-fc-subtext text-fc-muted"><Spinner />{t('capture.starting')}</p>
		{:else if phase === 'failed'}
			<Alert tone="danger" title={t('live.mediaProblem')}>{t('capture.modelFailed')}</Alert>
		{/if}
	{/if}
</div>
