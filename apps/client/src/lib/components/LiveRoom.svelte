<script lang="ts">
	import { Room, RoomEvent, type LocalParticipant, type RemoteParticipant } from 'livekit-client';
	import { MediaQuery } from 'svelte/reactivity';
	import { Alert } from '@facile/muse';
	import { t } from '$lib/i18n/index.svelte';
	import { parseChat } from '$lib/roomData';
	import { bindRemoteAudio } from '$lib/roomAudio';
	import { canShareScreen, isSharing, setScreenShare } from '$lib/screenShare';
	import { connectEncrypted, decryptError, encryption, isE2EESupported } from '$lib/e2ee';
	import ParticipantTile from './ParticipantTile.svelte';
	import Stage from './Stage.svelte';
	import RoomHeader from './RoomHeader.svelte';
	import AudioPrompt from './AudioPrompt.svelte';
	import ControlsBar from './ControlsBar.svelte';
	import ChatDock from './ChatDock.svelte';
	import type { ChatMessage } from './ChatPanel.svelte';
	import CallCountdown from './CallCountdown.svelte';
	import CaptureBanner from './CaptureBanner.svelte';
	import { CaptureSession } from '$lib/captureSession.svelte';
	import { FocusState } from '$lib/focusState.svelte';

	interface Props {
		url: string;
		token: string;
		e2eeKey: string;
		displayName: string;
		slug?: string;
		historyHref?: string;
		embedded?: boolean;
		callDeadline?: string;
		deadlineKey?: string;
		seatKey?: string;
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
		deadlineKey = '',
		seatKey = '',
		onLeave
	}: Props = $props();

	let room: Room | null = null;
	let participants = $state<(LocalParticipant | RemoteParticipant)[]>([]);
	let micOn = $state(false);
	let camOn = $state(false);
	let screenOn = $state(false);
	const wide = new MediaQuery('min-width: 768px');
	let chatOpen = $state(wide.current);
	let unread = $state(0);
	let selfId = $state('');
	let version = $state(0);
	let audioBlocked = $state(false);
	let messages = $state<ChatMessage[]>([]);
	let error = $state('');
	let screenError = $state('');
	let cryptError = $state('');
	const failing = new Set<string>();
	let encrypted = $state(false);
	let capture = $state<CaptureSession | null>(null);

	const supported = isE2EESupported();
	const encoder = new TextEncoder();
	const decoder = new TextDecoder();

	const selfAside = $derived(chatOpen && wide.current && participants.length > 1);
	const tiles = $derived(selfAside ? participants.filter((p) => !p.isLocal) : participants);
	const self = $derived(participants.find((p) => p.isLocal));
	const stage = new FocusState(() => {
		void version;
		return participants.map((p) => ({ identity: p.identity, isLocal: p.isLocal, sharing: isSharing(p) }));
	});

	$effect(() => stage.sync());

	function refresh() {
		if (!room)
			return;

		version++;
		selfId = room.localParticipant.identity;
		participants = [room.localParticipant, ...Array.from(room.remoteParticipants.values())];
		capture?.reconcile();
		micOn = room.localParticipant.isMicrophoneEnabled;
		camOn = room.localParticipant.isCameraEnabled;
		screenOn = room.localParticipant.isScreenShareEnabled;
		encrypted = room.isE2EEEnabled;
	}

	function handleData(payload: Uint8Array<ArrayBuffer>, from?: RemoteParticipant) {
		const message = parseChat(decoder.decode(payload), from?.name ?? from?.identity);
		if (!message) return;
		messages = [...messages, message];
		if (!chatOpen) unread++;
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

	async function leave() {
		await capture?.end();
		await room?.disconnect();
		if (onLeave) onLeave();
		else window.location.href = '/';
	}

	function publishMedia(r: Room) {
		refresh();
		Promise.all([r.localParticipant.setCameraEnabled(true), r.localParticipant.setMicrophoneEnabled(true)])
			.catch(() => (error = t('live.noMedia')))
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

		const session = slug && seatKey && !embedded ? new CaptureSession(slug, seatKey, () => r) : null;
		connectEncrypted(r, keyProvider, e2eeKey, { url, token })
			.then(() => {
				publishMedia(r);
				capture = session;
				session?.begin();
			})
			.catch((e: unknown) => (error = e instanceof Error ? e.message : String(e)));

		return () => {
			void session?.end();
			capture = null;
			r.disconnect().finally(() => worker.terminate());
			unbindAudio();
			room = null;
		};
	});
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (chatOpen && !wide.current ? (chatOpen = false) : stage.unpin())} />

{#if !supported}
	<main class="flex min-h-dvh items-center justify-center px-4">
		<Alert tone="danger" title={t('live.browserTitle')}>{t('live.browserBody')}</Alert>
	</main>
{:else}
<div class="flex h-dvh flex-col">
	{#if !embedded}
		<RoomHeader {slug} {historyHref} count={participants.length} />
	{/if}

	{#if callDeadline || (slug && !embedded)}
		<CallCountdown deadline={callDeadline} {slug} {deadlineKey} />
	{/if}

	<AudioPrompt blocked={audioBlocked} onEnable={() => void room?.startAudio()} />

	{#if capture?.state && (capture.state.transcription || capture.state.recording)}
		<CaptureBanner session={capture} />
	{/if}
	{#if cryptError || error || screenError || capture?.error}
		<div class="px-4 pb-3">
			<Alert tone={cryptError ? 'danger' : 'warning'} title={cryptError ? t('live.cannotDecrypt') : t('live.mediaProblem')}>
				{cryptError || error || screenError || capture?.error}
			</Alert>
		</div>
	{/if}

	<div class="flex min-h-0 flex-1 pl-4 pr-1">
		<Stage {participants} {tiles} {version} focus={stage.focus} onselect={(f) => stage.select(f)} onunpin={() => stage.unpin()} />

		<ChatDock open={chatOpen} wide={wide.current} {messages} {selfId} onSend={sendChat} onClose={() => (chatOpen = false)}>
			{#snippet top()}
				{#if selfAside && self}
					<ParticipantTile participant={self} isLocal {version} onselect={() => stage.select({ identity: self.identity, source: 'camera' })} />
				{/if}
			{/snippet}
		</ChatDock>
	</div>

	<footer class="flex justify-center pt-1 pb-1">
		<ControlsBar
			{micOn}
			{camOn}
			{screenOn}
			{chatOpen}
			{unread}
			{encrypted}
			canShare={canShareScreen()}
			{capture}
			onToggleMic={toggleMic}
			onToggleCam={toggleCam}
			onToggleScreen={toggleScreen}
			onToggleChat={() => ((chatOpen = !chatOpen), (unread = 0))}
			onLeave={leave}
		/>
	</footer>
</div>
{/if}
