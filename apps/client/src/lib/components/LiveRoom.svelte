<script lang="ts">
	import {
		Room,
		RoomEvent,
		Track,
		type LocalParticipant,
		type RemoteParticipant
	} from 'livekit-client';
	import { MediaQuery } from 'svelte/reactivity';
	import gsap from 'gsap';
	import { Alert, prefersReducedMotion } from '@facile/muse';
	import { parseCaption, parseChat } from '$lib/roomData';
	import { gridCols, gridColumn, gridRows, tileWidth } from '$lib/grid';
	import { bindRemoteAudio } from '$lib/roomAudio';
	import { ApiError, startRecording, stopRecording } from '$lib/api';
	import ParticipantTile from './ParticipantTile.svelte';
	import RoomHeader from './RoomHeader.svelte';
	import AudioPrompt from './AudioPrompt.svelte';
	import ControlsBar from './ControlsBar.svelte';
	import ChatPanel, { type ChatMessage } from './ChatPanel.svelte';
	import CaptionsOverlay, { type Caption } from './CaptionsOverlay.svelte';

	interface Props {
		url: string;
		token: string;
		displayName: string;
		slug?: string;
		historyHref?: string;
	}

	let { url, token, displayName, slug = '', historyHref = '' }: Props = $props();

	let room: Room | null = null;
	let participants = $state<(LocalParticipant | RemoteParticipant)[]>([]);
	let micOn = $state(false);
	let camOn = $state(false);
	let screenOn = $state(false);
	let chatOpen = $state(true);
	let version = $state(0);
	let audioBlocked = $state(false);
	let messages = $state<ChatMessage[]>([]);
	let captions = $state<Caption[]>([]);
	let captionId = 0;
	let recording = $state(false);
	let recordError = $state('');
	let error = $state('');

	const encoder = new TextEncoder();
	const decoder = new TextDecoder();

	const canRecord = $derived(Boolean(historyHref));

	let asideEl: HTMLElement | undefined = $state();
	let asideShown = false;

	$effect(() => {
		const open = chatOpen;

		if (!asideEl)
			return;

		const instant = !asideShown || prefersReducedMotion();

		asideShown = true;
		gsap.to(asideEl, {
			width: open ? 'auto' : 0,
			opacity: open ? 1 : 0,
			x: open ? 0 : 24,
			duration: instant ? 0 : 0.45,
			ease: 'power3.inOut',
		});
	});

	const wide = new MediaQuery('min-width: 768px');
	const selfAside = $derived(chatOpen && wide.current && participants.length > 1);
	const tiles = $derived(selfAside ? participants.filter((p) => !p.isLocal) : participants);
	const self = $derived(participants.find((p) => p.isLocal));

	const rows = $derived(gridRows(tiles.length));
	const cols = $derived(gridCols(tiles.length, rows));
	const width = $derived(tileWidth(rows));

	function refresh() {
		if (!room)
			return;

		version++;
		participants = [room.localParticipant, ...Array.from(room.remoteParticipants.values())];
		micOn = room.localParticipant.isMicrophoneEnabled;
		camOn = room.localParticipant.isCameraEnabled;
		screenOn =
			room.localParticipant.getTrackPublication(Track.Source.ScreenShare)?.isMuted === false;
	}

	function handleData(
		payload: Uint8Array<ArrayBuffer>,
		from?: RemoteParticipant,
		_kind?: unknown,
		topic?: string
	) {
		const text = decoder.decode(payload);

		if (topic === 'transcription') {
			const caption = parseCaption(text, ++captionId);
			if (caption) captions = [...captions.slice(-20), caption];
			return;
		}

		const message = parseChat(text, from?.name ?? from?.identity);
		if (message) messages = [...messages, message];
	}

	function sendChat(text: string) {
		if (!room) return;
		const msg: ChatMessage = { from: room.localParticipant.identity, name: displayName, text, at: Date.now() };
		room.localParticipant.publishData(encoder.encode(JSON.stringify(msg)), { reliable: true });
		messages = [...messages, msg];
	}

	async function toggleMic() {
		await room?.localParticipant.setMicrophoneEnabled(!micOn);
		refresh();
	}

	async function toggleCam() {
		await room?.localParticipant.setCameraEnabled(!camOn);
		refresh();
	}

	async function toggleScreen() {
		await room?.localParticipant.setScreenShareEnabled(!screenOn);
		refresh();
	}

	function leave() {
		room?.disconnect();
		window.location.href = '/';
	}

	async function toggleRecording() {
		if (!slug) return;
		recordError = '';
		try {
			if (recording) {
				await stopRecording(slug);
				recording = false;
			} else {
				await startRecording(slug);
				recording = true;
			}
		} catch (e) {
			const message = e instanceof Error ? e.message : String(e);
			const status = e instanceof ApiError ? e.status : 0;
			if (!recording && status === 409) {
				recording = true;
				return;
			}
			recordError = message;
		}
	}

	$effect(() => {
		const r = new Room({ adaptiveStream: true, dynacast: true });
		room = r;
		r.on(RoomEvent.ParticipantConnected, refresh)
			.on(RoomEvent.ParticipantDisconnected, refresh)
			.on(RoomEvent.TrackSubscribed, refresh)
			.on(RoomEvent.TrackUnsubscribed, refresh)
			.on(RoomEvent.LocalTrackPublished, refresh)
			.on(RoomEvent.LocalTrackUnpublished, refresh)
			.on(RoomEvent.TrackMuted, refresh)
			.on(RoomEvent.TrackUnmuted, refresh)
			.on(RoomEvent.ActiveSpeakersChanged, refresh)
			.on(RoomEvent.DataReceived, handleData);

		const unbindAudio = bindRemoteAudio(r, (blocked) => (audioBlocked = blocked));

		r.connect(url, token)
			.then(async () => {
				refresh();
				try {
					await r.localParticipant.setCameraEnabled(true);
					await r.localParticipant.setMicrophoneEnabled(true);
				} catch {
					error = 'Camera or microphone unavailable. You joined without publishing.';
				}
				refresh();
			})
			.catch((e: unknown) => {
				error = e instanceof Error ? e.message : String(e);
			});

		return () => {
			r.disconnect();
			unbindAudio();
			room = null;
		};
	});
</script>

<div class="flex h-dvh flex-col">
	<RoomHeader {slug} {recording} {historyHref} count={participants.length} />

	<AudioPrompt blocked={audioBlocked} onEnable={() => void room?.startAudio()} />

	{#if error || recordError}
		<div class="px-4 pb-3">
			<Alert tone={error ? 'warning' : 'danger'} title={error ? 'Joined without media' : 'Recording failed'}>
				{error || recordError}
			</Alert>
		</div>
	{/if}

	<div class="flex min-h-0 flex-1 pl-4 pr-1">
		<main
			class="grid min-h-0 flex-1 content-center gap-1 [container-type:size]"
			style:grid-template-columns="repeat({cols * 2}, minmax(0, 1fr))"
		>
			{#each tiles as p, i (p.identity)}
				<div class="flex min-w-0 justify-center" style:grid-column={gridColumn(i, tiles.length, rows, cols)}>
					<div style:width={width}>
						<ParticipantTile participant={p} isLocal={p.isLocal} {version} />
					</div>
				</div>
			{:else}
				<p class="col-span-full self-center text-center text-fc-p text-fc-muted">Connecting…</p>
			{/each}
		</main>

		<aside bind:this={asideEl} class="hidden shrink-0 overflow-hidden md:block" aria-hidden={!chatOpen} inert={!chatOpen}>
			<div class="flex h-full w-81 flex-col gap-1 pl-1">
				{#if selfAside && self}
					<ParticipantTile participant={self} isLocal {version} />
				{/if}
				<ChatPanel {messages} open={chatOpen} onSend={sendChat} />
			</div>
		</aside>
	</div>

	<CaptionsOverlay bind:captions />

	<footer class="flex justify-center pt-1 pb-1">
		<ControlsBar
			{micOn}
			{camOn}
			{screenOn}
			{chatOpen}
			{recording}
			{canRecord}
			onToggleMic={toggleMic}
			onToggleCam={toggleCam}
			onToggleScreen={toggleScreen}
			onToggleChat={() => (chatOpen = !chatOpen)}
			onToggleRecording={() => void toggleRecording()}
			onLeave={leave}
		/>
	</footer>
</div>
