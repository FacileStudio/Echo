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

	const camera = $derived.by(() => {
		void version;
		return participant.getTrackPublication(Track.Source.Camera)?.videoTrack ?? null;
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
		'relative isolate aspect-video overflow-hidden rounded-fc border border-transparent bg-fc-surface',
		speaking && 'border-1 border-fc-brand'
	)}
>
	{#if camera}
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
		{#if !micOn}
			<Icon icon={callIcons.micOff} size={14} class="shrink-0 text-fc-danger" aria-label="Muted" />
		{/if}
		<span class="truncate text-fc-subtext text-fc-ink">{name}{isLocal ? ' (you)' : ''}</span>
	</div>
</div>
