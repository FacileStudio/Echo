<script lang="ts">
	import { Room, RoomEvent, type LocalParticipant, type RemoteParticipant } from 'livekit-client';
	import { MediaQuery } from 'svelte/reactivity';
	import gsap from 'gsap';
	import { Alert, prefersReducedMotion } from '@facile/muse';
	import { parseChat } from '$lib/roomData';
	import { bindRemoteAudio } from '$lib/roomAudio';
	import { canShareScreen, setScreenShare } from '$lib/screenShare';
	import { connectEncrypted, decryptError, encryption, isE2EESupported } from '$lib/e2ee';
	import ParticipantTile from './ParticipantTile.svelte';
	import Stage from './Stage.svelte';
	import RoomHeader from './RoomHeader.svelte';
	import AudioPrompt from './AudioPrompt.svelte';
	import ControlsBar from './ControlsBar.svelte';
	import ChatPanel, { type ChatMessage } from './ChatPanel.svelte';
	import CallCountdown from './CallCountdown.svelte';

	interface Props {
		url: string;
		token: string;
		e2eeKey: string;
		displayName: string;
		slug?: string;
		historyHref?: string;
		embedded?: boolean;
		callDeadline?: string;
		onLeave?: () => void;
	}

	let {
		url,
		token,
		e2eeKey,
		displayName,
		slug = '',
		historyHref = '',
		embedded = false,
		callDeadline = '',
		onLeave
	}: Props = $props();

	let room: Room | null = null;
	let participants = $state<(LocalParticipant | RemoteParticipant)[]>([]);
	let micOn = $state(false);
	let camOn = $state(false);
	let screenOn = $state(false);
	let chatOpen = $state(true);
	let version = $state(0);
	let audioBlocked = $state(false);
	let messages = $state<ChatMessage[]>([]);
	let error = $state('');
	let screenError = $state('');
	let cryptError = $state('');
	const failing = new Set<string>();
	let encrypted = $state(false);

	const supported = isE2EESupported();

	const encoder = new TextEncoder();
	const decoder = new TextDecoder();

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

	function refresh() {
		if (!room)
			return;

		version++;
		participants = [room.localParticipant, ...Array.from(room.remoteParticipants.values())];
		micOn = room.localParticipant.isMicrophoneEnabled;
		camOn = room.localParticipant.isCameraEnabled;
		screenOn = room.localParticipant.isScreenShareEnabled;
		encrypted = room.isE2EEEnabled;
	}

	function handleData(payload: Uint8Array<ArrayBuffer>, from?: RemoteParticipant) {
		const message = parseChat(decoder.decode(payload), from?.name ?? from?.identity);
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
		if (!room) return;
		screenError = await setScreenShare(room.localParticipant, !screenOn);
		refresh();
	}

	function leave() {
		room?.disconnect();
		if (onLeave) onLeave();
		else window.location.href = '/';
	}

	function publishMedia(r: Room) {
		refresh();
		Promise.all([r.localParticipant.setCameraEnabled(true), r.localParticipant.setMicrophoneEnabled(true)])
			.catch(() => (error = 'Camera or microphone unavailable. You joined without publishing.'))
			.finally(refresh);
	}

	$effect(() => {
		if (!supported) return;

		const { keyProvider, worker } = encryption();
		const r = new Room({ adaptiveStream: true, dynacast: true, encryption: { keyProvider, worker } });
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
			.on(RoomEvent.ParticipantEncryptionStatusChanged, refresh)
			.on(RoomEvent.EncryptionError, (_e, p) => {
				if (p) failing.add(p.name || p.identity);
				cryptError = decryptError([...failing], r.remoteParticipants.size);
			})
			.on(RoomEvent.DataReceived, handleData);

		const unbindAudio = bindRemoteAudio(r, (blocked) => (audioBlocked = blocked));

		connectEncrypted(r, keyProvider, e2eeKey, { url, token })
			.then(() => publishMedia(r))
			.catch((e: unknown) => (error = e instanceof Error ? e.message : String(e)));

		return () => {
			r.disconnect().finally(() => worker.terminate());
			unbindAudio();
			room = null;
		};
	});
</script>

{#if !supported}
	<main class="flex min-h-dvh items-center justify-center px-4">
		<Alert tone="danger" title="This browser cannot join">
			Echo calls are end-to-end encrypted, and this browser cannot encrypt media. Open the link in a
			recent Chrome, Edge, Firefox or Safari.
		</Alert>
	</main>
{:else}
<div class="flex h-dvh flex-col">
	{#if !embedded}
		<RoomHeader {slug} {historyHref} count={participants.length} />
	{/if}

	{#if callDeadline || (slug && !embedded)}
		<CallCountdown deadline={callDeadline} {slug} />
	{/if}

	<AudioPrompt blocked={audioBlocked} onEnable={() => void room?.startAudio()} />

	{#if cryptError || error || screenError}
		<div class="px-4 pb-3">
			<Alert tone={cryptError ? 'danger' : 'warning'} title={cryptError ? 'Cannot decrypt' : 'Media problem'}>
				{cryptError || error || screenError}
			</Alert>
		</div>
	{/if}

	<div class="flex min-h-0 flex-1 pl-4 pr-1">
		<Stage {participants} {tiles} {version} />

		<aside bind:this={asideEl} class="hidden shrink-0 overflow-hidden md:block" aria-hidden={!chatOpen} inert={!chatOpen}>
			<div class="flex h-full w-81 flex-col gap-1 pl-1">
				{#if selfAside && self}
					<ParticipantTile participant={self} isLocal {version} />
				{/if}
				<ChatPanel {messages} open={chatOpen} onSend={sendChat} />
			</div>
		</aside>
	</div>

	<footer class="flex justify-center pt-1 pb-1">
		<ControlsBar
			{micOn}
			{camOn}
			{screenOn}
			{chatOpen}
			{encrypted}
			canShare={canShareScreen()}
			onToggleMic={toggleMic}
			onToggleCam={toggleCam}
			onToggleScreen={toggleScreen}
			onToggleChat={() => (chatOpen = !chatOpen)}
			onLeave={leave}
		/>
	</footer>
</div>
{/if}
