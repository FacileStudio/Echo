<script lang="ts">
	import { Track, type LocalParticipant, type RemoteParticipant } from 'livekit-client';
	import { Icon, cn } from '@facile/muse';
	import { t } from '$lib/i18n/index.svelte';
	import { callIcons } from '$lib/icons';

	interface Props {
		participant: LocalParticipant | RemoteParticipant;
		isLocal?: boolean;
		screen?: boolean;
		version: number;
	}

	let { participant, isLocal = false, screen = false, version }: Props = $props();

	let videoEl: HTMLVideoElement | undefined = $state();

	const camera = $derived.by(() => {
		void version;
		const source = screen ? Track.Source.ScreenShare : Track.Source.Camera;
		return participant.getTrackPublication(source)?.videoTrack ?? null;
	});
	const micOn = $derived.by(() => {
		void version;
		return participant.isMicrophoneEnabled;
	});
	const speaking = $derived.by(() => {
		void version;
		return participant.isSpeaking;
	});

	const name = $derived(participant.name || participant.identity);
	const label = $derived(
		screen ? (isLocal ? t('tile.yourScreen') : t('tile.screenOf', { name })) : isLocal ? t('tile.you', { name }) : name
	);

	const avatar = $derived.by(() => {
		void version;
		try {
			const url = (JSON.parse(participant.metadata || '{}') as { avatar_url?: unknown }).avatar_url;
			return typeof url === 'string' && /^https?:\/\//.test(url) ? url : '';
		} catch {
			return '';
		}
	});

	const initials = $derived(
		name
			.split(/\s+/)
			.map((w) => w[0])
			.filter(Boolean)
			.slice(0, 2)
			.join('')
			.toUpperCase() || '?'
	);

	$effect(() => {
		const track = camera;
		const el = videoEl;

		if (!track || !el)
			return;

		track.attach(el);
		return () => track.detach(el);
	});
</script>

<div
	class={cn(
		'relative isolate overflow-hidden rounded-fc border border-transparent bg-fc-surface',
		screen ? 'size-full' : 'aspect-video',
		speaking && !screen && 'border-1 border-fc-brand'
	)}
>
	{#if camera}
		<video
			bind:this={videoEl}
			autoplay
			playsinline
			muted={isLocal}
			class={cn('size-full rounded-fc', screen ? 'object-contain' : 'object-cover', isLocal && !screen && '-scale-x-100')}
		></video>
	{:else}
		<div class="flex size-full items-center justify-center">
			{#if avatar}
				<img src={avatar} alt="" referrerpolicy="no-referrer" class="size-20 rounded-full object-cover" />
			{:else}
				<span class="fc-selected flex size-20 items-center justify-center rounded-full text-fc-subtitle">
					{initials}
				</span>
			{/if}
		</div>
	{/if}

	<div class="absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] items-center gap-1 rounded-fc bg-fc-scrim px-2 py-1">
		{#if !micOn && !screen}
			<Icon icon={callIcons.micOff} size={14} class="shrink-0 text-fc-danger" aria-label={t('tile.muted')} />
		{/if}
		<span class="truncate text-fc-subtext text-fc-ink">{label}</span>
	</div>
</div>
