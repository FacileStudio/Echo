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
	import { Alert, Badge, Button, Icon, IconButton, prefersReducedMotion } from '@facile/muse';
	import { callIcons } from '$lib/icons';
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
	let copied = $state(false);
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

	// favour rows over columns: two people stack, three make 2 + 1
	const rows = $derived(Math.max(1, Math.ceil(Math.sqrt(tiles.length))));
	const cols = $derived(Math.max(1, Math.ceil(tiles.length / rows)));

	// a tile is as wide as its cell, unless that makes the rows taller than the grid: then the row height (grid height minus the gaps, split across the rows) times 16/9
	const tileWidth = $derived(`min(100%, calc((100cqh - ${rows - 1} * 0.25rem) / ${rows} * 16 / 9))`);

	// each tile spans two half-tracks, so a short last row can start half a tile in and sit centred
	function column(i: number) {
		const lastRowStart = (rows - 1) * cols;
		const remainder = tiles.length - lastRowStart;
		// remaining tiles leave (cols - remainder) empty slots, half of them on each side
		const offset = i === lastRowStart && remainder < cols ? cols - remainder + 1 : 'auto';

		return `${offset} / span 2`;
	}

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

	async function copyLink() {
		await navigator.clipboard.writeText(window.location.href);
		copied = true;
		setTimeout(() => (copied = false), 2000);
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

<div class="flex h-dvh flex-col">
	<header class="flex items-center justify-between gap-1 px-4 py-3">
		<div class="flex min-w-0 items-center gap-1">
			<a href="/" class="flex items-center gap-1 text-fc-lead text-fc-ink">
				<img src="/logo.svg" alt="" class="size-7" />
				Echo
			</a>
			<span class="truncate text-fc-p text-fc-muted">{slug}</span>
			{#if recording}
				<Badge tone="danger">Recording</Badge>
			{/if}
		</div>
		<div class="flex shrink-0 items-center gap-1">
			<span class="flex items-center gap-1 px-2 text-fc-p text-fc-muted" aria-label="{participants.length} in the call">
				<Icon icon={callIcons.people} size={16} />
				{participants.length}
			</span>
			<Button variant="ghost" size="sm" icon={callIcons.link} onclick={() => void copyLink()}>
				{copied ? 'Copied' : 'Copy link'}
			</Button>
			{#if historyHref}
				<Button variant="ghost" size="sm" href={historyHref}>History</Button>
			{/if}
		</div>
	</header>

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
				<div class="flex min-w-0 justify-center" style:grid-column={column(i)}>
					<div style:width={tileWidth}>
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
				<ChatPanel {messages} open={chatOpen} onSend={sendChat} onClose={() => (chatOpen = false)} />
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
