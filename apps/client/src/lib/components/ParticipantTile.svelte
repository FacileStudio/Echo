<script lang="ts">
	import { Track, type LocalParticipant, type RemoteParticipant } from 'livekit-client';
	import { Icon, cn } from '@facile/muse';
	import { callIcons } from '$lib/icons';

	interface Props {
		participant: LocalParticipant | RemoteParticipant;
		isLocal?: boolean;
		version: number;
	}

	let { participant, isLocal = false, version }: Props = $props();

	let videoEl: HTMLVideoElement | undefined = $state();

	// `version` bumps on every room event; the participant object itself never changes identity
	const live = $derived.by(() => {
		void version;
		return {
			camera: participant.getTrackPublication(Track.Source.Camera)?.videoTrack ?? null,
			micOn: participant.isMicrophoneEnabled,
			speaking: participant.isSpeaking
		};
	});

	const name = $derived(participant.name || participant.identity);

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
		const track = live.camera;
		const el = videoEl;

		if (!track || !el)
			return;

		track.attach(el);
		return () => track.detach(el);
	});
</script>

<div
	class={cn(
		'relative isolate aspect-video overflow-hidden rounded-fc border border-transparent bg-fc-surface',
		live.speaking && 'border-1 border-fc-brand'
	)}
>
	{#if live.camera}
		<video
			bind:this={videoEl}
			autoplay
			playsinline
			muted={isLocal}
			class={cn('size-full rounded-fc object-cover', isLocal && '-scale-x-100')}
		></video>
	{:else}
		<div class="flex size-full items-center justify-center">
			<span class="fc-selected flex size-20 items-center justify-center rounded-full text-fc-subtitle">
				{initials}
			</span>
		</div>
	{/if}

	<div class="absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] items-center gap-1 rounded-fc bg-fc-scrim px-2 py-1">
		{#if !live.micOn}
			<Icon icon={callIcons.micOff} size={14} class="shrink-0 text-fc-danger" aria-label="Muted" />
		{/if}
		<span class="truncate text-fc-subtext text-fc-ink">{name}{isLocal ? ' (you)' : ''}</span>
	</div>
</div>
