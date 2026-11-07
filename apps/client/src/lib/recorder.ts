import { putChunk, startUpload, finalizeUpload, finalizeOnExit, type Upload } from './recordingApi';

const candidates = ['audio/webm;codecs=opus', 'audio/ogg;codecs=opus', 'audio/mp4'];
const sliceMs = 5000;
const retryMs = [1000, 2000, 5000, 10000];

/** The container this browser's MediaRecorder writes, as the base type the API accepts. */
export function recorderFormat(): { mimeType: string; contentType: string } | null {
	if (typeof MediaRecorder === 'undefined') return null;
	const mimeType = candidates.find((type) => MediaRecorder.isTypeSupported(type));
	return mimeType ? { mimeType, contentType: mimeType.split(';')[0] } : null;
}

/**
 * Mixes the audio of the participants who agreed to be recorded into one stream. A track is
 * connected only while its identity is in the allowed set: `allow` is called again whenever the
 * server's consent state or the room changes, and a voice that is no longer allowed is
 * disconnected at once, before another sample is recorded.
 */
export class ConsentingMix {
	private context = new AudioContext();
	private destination = this.context.createMediaStreamDestination();
	private sources = new Map<string, { track: MediaStreamTrack; node: MediaStreamAudioSourceNode }>();

	get stream(): MediaStream {
		return this.destination.stream;
	}

	allow(allowed: readonly string[], tracks: ReadonlyMap<string, MediaStreamTrack>): void {
		for (const [identity, held] of this.sources) {
			if (!allowed.includes(identity) || tracks.get(identity) !== held.track) {
				held.node.disconnect();
				this.sources.delete(identity);
			}
		}
		for (const identity of allowed) {
			const track = tracks.get(identity);
			if (!track || this.sources.has(identity)) continue;
			const node = this.context.createMediaStreamSource(new MediaStream([track]));
			node.connect(this.destination);
			this.sources.set(identity, { track, node });
		}
	}

	resume(): Promise<void> {
		return this.context.resume();
	}

	close(): void {
		for (const held of this.sources.values()) held.node.disconnect();
		this.sources.clear();
		void this.context.close();
	}
}

/**
 * Records a stream with MediaRecorder and uploads it in order, 5 s at a time. A chunk that fails
 * is retried; when the server says it is behind (409), upload resumes from the chunk it names.
 * Chunks wait in memory until the server has acknowledged them.
 */
export class ChunkedRecording {
	private recorder: MediaRecorder | null = null;
	private upload: Upload | null = null;
	private queue: { index: number; data: Blob }[] = [];
	private next = 0;
	private sending = false;
	private failed = false;
	private stopped: (() => void) | null = null;

	constructor(
		private slug: string,
		private onFailure: () => void
	) {}

	get id(): string | null {
		return this.upload?.id ?? null;
	}

	async start(stream: MediaStream): Promise<void> {
		const format = recorderFormat();
		if (!format) throw new Error('MediaRecorder');
		this.upload = await startUpload(this.slug, format.contentType);
		this.next = this.upload.next_chunk;
		const recorder = new MediaRecorder(stream, { mimeType: format.mimeType });
		recorder.ondataavailable = (event) => {
			if (event.data.size > 0) this.queue.push({ index: this.next++, data: event.data });
			void this.drain();
		};
		recorder.onstop = () => this.stopped?.();
		recorder.start(sliceMs);
		this.recorder = recorder;
	}

	private async drain(): Promise<void> {
		if (this.sending || !this.upload) return;
		this.sending = true;
		let attempt = 0;
		while (this.queue.length > 0 && !this.failed) {
			const head = this.queue[0];
			const result = await putChunk(this.slug, this.upload.id, head.index, head.data).catch(() => null);
			if (result?.ok) {
				attempt = 0;
				this.queue = this.queue.filter((chunk) => chunk.index >= result.upload.next_chunk);
				continue;
			}
			if (result?.status === 409 && result.gap) {
				this.queue = this.queue.filter((chunk) => chunk.index >= result.gap!.next_chunk);
				if (!this.queue.some((chunk) => chunk.index === result.gap!.next_chunk)) this.failed = true;
				continue;
			}
			if (result && result.status >= 400 && result.status < 500) this.failed = true;
			else if (attempt >= retryMs.length) this.failed = true;
			else await new Promise((resolve) => setTimeout(resolve, retryMs[attempt++]));
		}
		this.sending = false;
		if (this.failed) this.onFailure();
	}

	async stop(): Promise<void> {
		const recorder = this.recorder;
		this.recorder = null;
		if (recorder && recorder.state !== 'inactive') {
			await new Promise<void>((resolve) => {
				this.stopped = resolve;
				recorder.stop();
			});
		}
		await this.drain();
		while (this.sending) await new Promise((resolve) => setTimeout(resolve, 100));
		if (this.upload && !this.failed) await finalizeUpload(this.slug, this.upload.id).catch(() => undefined);
	}

	exit(): void {
		if (this.upload) finalizeOnExit(this.slug, this.upload.id);
	}
}
