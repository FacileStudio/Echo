<script lang="ts">
	import {
		Room,
		RoomEvent,
		Track,
		type LocalParticipant,
		type RemoteParticipant
	} from 'livekit-client';
	import { Button, Card } from '@facile/muse';
	import { ApiError, startRecording, stopRecording } from '$lib/api';
	import ParticipantTile from './ParticipantTile.svelte';
	import ControlsBar from './ControlsBar.svelte';
	import ChatPanel, { type ChatMessage } from './ChatPanel.svelte';
	import CaptionsOverlay, { type Caption } from './CaptionsOverlay.svelte';

	interface Props {
		url: string;
		token: string;
		displayName: string;
		slug?: string;
	}

	let { url, token, displayName, slug = '' }: Props = $props();

	let room: Room | null = null;
	let participants = $state<(LocalParticipant | RemoteParticipant)[]>([]);
	let micOn = $state(false);
	let camOn = $state(false);
	let screenOn = $state(false);
	let chatOpen = $state(true);
	let messages = $state<ChatMessage[]>([]);
	let captions = $state<Caption[]>([]);
	let captionId = 0;
	let recording = $state(false);
	let recordError = $state('');
	let error = $state('');

	const encoder = new TextEncoder();
	const decoder = new TextDecoder();

	function refresh() {
		if (!room) return;
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
		if (topic === 'transcription') {
			try {
				const msg = JSON.parse(decoder.decode(payload)) as { speaker?: string; text?: string };
				if (!msg.text) return;
				captions = [
					...captions.slice(-20),
					{ id: ++captionId, speaker: String(msg.speaker ?? 'unknown'), text: String(msg.text) }
				];
			} catch {
				return;
			}
			return;
		}
		try {
			const msg = JSON.parse(decoder.decode(payload)) as Omit<ChatMessage, 'name'> & {
				name?: string;
			};
			messages = [
				...messages,
				{
					from: String(msg.from ?? ''),
					name: msg.name ?? from?.name ?? from?.identity ?? 'Unknown',
					text: String(msg.text ?? ''),
					at: Number(msg.at) || Date.now()
				}
			];
		} catch {
			return;
		}
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
			.on(RoomEvent.DataReceived, handleData);

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
			room = null;
		};
	});
</script>

<div class="flex h-dvh flex-col gap-4 p-4">
	<header class="flex items-center justify-between">
		<h1 class="text-fc-lg font-semibold text-fc-fg">{displayName}</h1>
		<Button variant="ghost" size="sm" onclick={() => (chatOpen = !chatOpen)}>
			{chatOpen ? 'Hide chat' : 'Show chat'}
		</Button>
	</header>

	{#if error}
		<Card><p class="text-fc-sm text-fc-danger">{error}</p></Card>
	{/if}

	<div class="flex min-h-0 flex-1 gap-4">
		<main class="grid min-h-0 flex-1 auto-rows-min content-start gap-3 overflow-y-auto sm:grid-cols-2 lg:grid-cols-3">
			{#each participants as p (p.identity)}
				<ParticipantTile participant={p} isLocal={p.isLocal} />
			{:else}
				<Card><p class="text-fc-sm text-fc-fg-muted">Connecting…</p></Card>
			{/each}
		</main>

		<CaptionsOverlay bind:captions />

		{#if chatOpen}
			<aside class="hidden w-80 shrink-0 rounded-fc-md bg-fc-component p-4 md:block">
				<h2 class="mb-3 text-fc-md font-semibold text-fc-fg">Chat</h2>
				<div class="h-[calc(100%-2.5rem)]">
					<ChatPanel {messages} onSend={sendChat} />
				</div>
			</aside>
		{/if}
	</div>

	<footer class="shrink-0 pb-2">
		<div class="mb-1 flex flex-col items-center gap-1">
			{#if recordError || recording}
				<p class="text-fc-sm {recordError ? 'text-fc-danger' : 'text-fc-fg-muted'}">
					{recordError ?? '● REC'}
				</p>
			{/if}
			<Button variant="ghost" size="sm" onclick={() => void toggleRecording()} disabled={!slug}>
				{recording ? 'Stop recording' : 'Record'}
			</Button>
		</div>
		<ControlsBar
			{micOn}
			{camOn}
			{screenOn}
			onToggleMic={toggleMic}
			onToggleCam={toggleCam}
			onToggleScreen={toggleScreen}
			onLeave={leave}
		/>
	</footer>
</div>
