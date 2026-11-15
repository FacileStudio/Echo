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
		focused?: boolean;
		onselect?: () => void;
		onunpin?: () => void;
	}

	let { participant, isLocal = false, screen = false, version, focused = false, onselect, onunpin }: Props = $props();

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

<div class={cn('group relative isolate', screen || focused ? 'size-full' : 'aspect-video')}>
<button
	type="button"
	class={cn(
		'relative block size-full overflow-hidden rounded-fc border border-transparent bg-fc-surface text-left',
		onselect ? 'cursor-pointer focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-fc-brand' : 'cursor-default',
		speaking && !screen && 'border-1 border-fc-brand'
	)}
	aria-pressed={onselect ? focused : undefined}
	aria-label={onselect ? (focused ? t('tile.unfocus', { name: label }) : t('tile.focus', { name: label })) : label}
	disabled={!onselect}
	onclick={onselect}
>
	{#if camera}
		<video
			bind:this={videoEl}
			autoplay
			playsinline
			muted={isLocal}
			class={cn('size-full rounded-fc', screen || focused ? 'object-contain' : 'object-cover', isLocal && !screen && '-scale-x-100')}
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
</button>
	{#if focused && onunpin}
		<button
			type="button"
			class="absolute top-2 right-2 flex items-center gap-1 rounded-fc bg-fc-scrim px-2 py-1 text-fc-subtext text-fc-ink hover:bg-fc-surface-hover focus-visible:outline-2 focus-visible:outline-fc-brand"
			onclick={onunpin}
		>
			<Icon icon={callIcons.close} size={14} />{t('tile.unpin')}
		</button>
	{/if}
</div>
