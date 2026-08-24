<script lang="ts">
	import { Track, type LocalParticipant, type RemoteParticipant } from 'livekit-client';
	import { cn } from '@facile/muse';

	interface Props {
		participant: LocalParticipant | RemoteParticipant;
		isLocal?: boolean;
	}

	let { participant, isLocal = false }: Props = $props();

	let videoEl: HTMLVideoElement | undefined = $state();

	const name = $derived(participant.name || participant.identity);
	const cameraTrack = $derived(
		participant.getTrackPublication(Track.Source.Camera)?.videoTrack ?? null
	);
	const micOn = $derived(participant.isMicrophoneEnabled);

	$effect(() => {
		const track = cameraTrack;
		const el = videoEl;
		if (!track || !el) return;
		track.attach(el);
		return () => track.detach(el);
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
</script>

<div
	class={cn(
		'relative aspect-video overflow-hidden rounded-fc-md bg-fc-surface',
		isLocal && 'ring-2 ring-fc-ring/40'
	)}
>
	{#if cameraTrack}
		<video bind:this={videoEl} autoplay playsinline muted={isLocal} class="size-full object-cover"></video>
	{:else}
		<div
			class="flex size-full items-center justify-center bg-fc-component text-fc-xl font-medium text-fc-fg-muted"
		>
			{initials}
		</div>
	{/if}

	<div class="absolute bottom-2 left-2 flex items-center gap-1.5 rounded-fc-md bg-fc-scrim px-2 py-1">
		<span class="block size-2 rounded-full {micOn ? 'bg-fc-success' : 'bg-fc-danger'}"></span>
		<span class="text-fc-xs text-white">{name}{isLocal ? ' (you)' : ''}</span>
	</div>
</div>
