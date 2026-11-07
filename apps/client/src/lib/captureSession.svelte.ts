import { Track, type Room } from 'livekit-client';
import { ApiError } from './api';
import {
	answerCapture,
	fetchCapture,
	leftOut,
	mayTranscribe,
	postUtterance,
	recordable,
	setCapture,
	type CaptureState
} from './capture';
import { t, locale } from './i18n/index.svelte';
import { PlanLimitError, planLimitText } from './planLimit';
import { ChunkedRecording, ConsentingMix, recorderFormat } from './recorder';
import {
	Transcriber,
	defaultSpeechLanguage,
	downloadModel,
	type SpeechLanguage,
	type SpeechPhase
} from './stt';

const pollMs = 3000;

/**
 * One browser's share of in-call transcription and recording. It polls what the organizer turned
 * on, and acts only on what this person accepted: it transcribes their own microphone when they
 * said yes, and, in the organizer's browser, records the mix of the voices that said yes.
 */
export class CaptureSession {
	state = $state<CaptureState | null>(null);
	phase = $state<SpeechPhase>('idle');
	progress = $state(0);
	language = $state<SpeechLanguage>(defaultSpeechLanguage(locale()));
	left = $state(0);
	error = $state('');

	private transcriber: Transcriber | null = null;
	private transcribing: MediaStreamTrack | null = null;
	private timer: ReturnType<typeof setInterval> | null = null;
	private recording: ChunkedRecording | null = null;
	private mix: ConsentingMix | null = null;
	private starting = false;
	private ended = false;

	constructor(
		private slug: string,
		private seatKey: string,
		private room: () => Room | null
	) {}

	begin(): void {
		void this.refresh();
		this.timer = setInterval(() => void this.refresh(), pollMs);
		addEventListener('pagehide', this.exit);
	}

	private exit = () => this.recording?.exit();

	async end(): Promise<void> {
		this.ended = true;
		if (this.timer) clearInterval(this.timer);
		removeEventListener('pagehide', this.exit);
		this.transcriber?.dispose();
		this.transcriber = null;
		await this.stopRecording();
	}

	async refresh(): Promise<void> {
		if (this.ended) return;
		const next = await fetchCapture(this.slug, this.seatKey).catch(() => null);
		if (this.ended || !next) return;
		this.state = next;
		this.reconcile();
	}

	reconcile(): void {
		const state = this.state;
		const room = this.room();
		if (!state || !room || this.ended) return;
		this.reconcileTranscription(state, room);
		this.reconcileRecording(state, room);
	}

	private micTrack(room: Room): MediaStreamTrack | null {
		return room.localParticipant.getTrackPublication(Track.Source.Microphone)?.track?.mediaStreamTrack ?? null;
	}

	private reconcileTranscription(state: CaptureState, room: Room): void {
		if (!mayTranscribe(state)) {
			this.transcriber?.stop();
			this.transcribing = null;
			return;
		}
		if (this.phase === 'idle') void this.loadSpeech();
		const track = this.micTrack(room);
		if (!this.transcriber || !track || track === this.transcribing) return;
		this.transcribing = track;
		this.transcriber.start(track, (line) => {
			if (!mayTranscribe(this.state)) return;
			void postUtterance(this.slug, this.seatKey, { text: line.text, start_ms: line.startMs, end_ms: line.endMs }).catch(
				() => undefined
			);
		});
	}

	private async loadSpeech(): Promise<void> {
		if (this.phase !== 'idle' && this.phase !== 'failed') return;
		this.phase = 'downloading';
		this.progress = 0;
		try {
			const archive = await downloadModel(this.language, (fraction) => (this.progress = fraction));
			this.phase = 'starting';
			const transcriber = await Transcriber.create(archive);
			if (this.ended) return transcriber.dispose();
			this.transcriber = transcriber;
			this.phase = 'ready';
		} catch {
			this.phase = 'failed';
		}
		this.reconcile();
	}

	private reconcileRecording(state: CaptureState, room: Room): void {
		if (!state.owner) return;
		if (!state.recording) {
			void this.stopRecording();
			return;
		}
		const present = [room.localParticipant, ...room.remoteParticipants.values()];
		const tracks = new Map<string, MediaStreamTrack>();
		for (const participant of present) {
			const track = participant.getTrackPublication(Track.Source.Microphone)?.track?.mediaStreamTrack;
			if (track) tracks.set(participant.identity, track);
		}
		const identities = present.map((participant) => participant.identity);
		this.left = leftOut(state, identities);
		if (!this.recording && !this.starting) void this.startRecording();
		this.mix?.allow(recordable(state, identities), tracks);
	}

	private async startRecording(): Promise<void> {
		if (!recorderFormat()) {
			this.error = t('capture.cannotRecord');
			return;
		}
		this.starting = true;
		try {
			const mix = new ConsentingMix();
			void mix.resume();
			const recording = new ChunkedRecording(this.slug, () => (this.error = t('capture.uploadFailed')));
			await recording.start(mix.stream);
			this.mix = mix;
			this.recording = recording;
			this.reconcile();
		} catch (e) {
			this.error = this.describe(e);
		} finally {
			this.starting = false;
		}
	}

	private async stopRecording(): Promise<void> {
		const recording = this.recording;
		const mix = this.mix;
		this.recording = null;
		this.mix = null;
		this.left = 0;
		await recording?.stop();
		mix?.close();
	}

	private describe(e: unknown): string {
		if (e instanceof PlanLimitError) return planLimitText(e);
		if (e instanceof ApiError) return e.message;
		return t('capture.failed');
	}

	async toggle(kind: 'transcription' | 'recording'): Promise<void> {
		this.error = '';
		const next = !(kind === 'transcription' ? this.state?.transcription : this.state?.recording);
		try {
			await setCapture(this.slug, this.seatKey, { [kind]: next });
		} catch (e) {
			this.error = this.describe(e);
		}
		await this.refresh();
	}

	async answer(answer: { transcribe?: boolean; record?: boolean }): Promise<void> {
		this.error = '';
		try {
			await answerCapture(this.slug, this.seatKey, answer);
		} catch (e) {
			this.error = this.describe(e);
		}
		await this.refresh();
	}

	chooseLanguage(language: SpeechLanguage): void {
		if (this.phase === 'idle' || this.phase === 'failed') this.language = language;
	}
}
