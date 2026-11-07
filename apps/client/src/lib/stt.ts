import type { Model } from 'vosk-browser';

export type SpeechLanguage = 'fr' | 'en';

export type SpeechPhase = 'idle' | 'downloading' | 'starting' | 'ready' | 'failed';

export const speechLanguages: readonly SpeechLanguage[] = ['fr', 'en'];

/** French stays French; every other session language falls back to English, the second model. */
export function defaultSpeechLanguage(locale: string): SpeechLanguage {
	return locale.slice(0, 2).toLowerCase() === 'fr' ? 'fr' : 'en';
}

const flushMs = 1500;
const cacheName = 'echo-stt-v1';
const modelUrl = (language: SpeechLanguage) => `/models/${language}.tar.gz`;

async function cached(url: string): Promise<Response | undefined> {
	try {
		return await (await caches.open(cacheName)).match(url);
	} catch {
		return undefined;
	}
}

async function remember(url: string, body: Blob): Promise<void> {
	try {
		await (await caches.open(cacheName)).put(url, new Response(body));
	} catch {
		return;
	}
}

/**
 * Downloads the model archive of a language once, reporting 0..1 as it arrives, and keeps it in
 * the browser's cache so the next call starts at once. Nothing is fetched before this is called.
 */
export async function downloadModel(language: SpeechLanguage, onProgress: (fraction: number) => void): Promise<Blob> {
	const url = modelUrl(language);
	const hit = await cached(url);
	if (hit) {
		onProgress(1);
		return hit.blob();
	}
	const res = await fetch(url);
	if (!res.ok || !res.body) throw new Error(`model ${res.status}`);
	const total = Number(res.headers.get('Content-Length')) || 0;
	const reader = res.body.getReader();
	const parts: Uint8Array<ArrayBuffer>[] = [];
	let received = 0;
	for (;;) {
		const { done, value } = await reader.read();
		if (done) break;
		parts.push(value as Uint8Array<ArrayBuffer>);
		received += value.byteLength;
		if (total > 0) onProgress(Math.min(1, received / total));
	}
	const blob = new Blob(parts, { type: 'application/gzip' });
	await remember(url, blob);
	onProgress(1);
	return blob;
}

export interface Finals {
	(line: { text: string; startMs: number; endMs: number }): void;
}

/**
 * On-device speech recognition with Vosk compiled to WebAssembly (vosk-browser), running in a
 * worker. The microphone track never leaves the page: only the final text lines do.
 */
export class Transcriber {
	private context: AudioContext | null = null;
	private processor: ScriptProcessorNode | null = null;
	private source: MediaStreamAudioSourceNode | null = null;
	private recognizer: InstanceType<Model['KaldiRecognizer']> | null = null;
	private spokenFrom = 0;

	private constructor(private model: Model) {}

	static async create(archive: Blob): Promise<Transcriber> {
		const { createModel } = await import('vosk-browser');
		const blobUrl = URL.createObjectURL(archive);
		try {
			return new Transcriber(await createModel(blobUrl, -2));
		} finally {
			URL.revokeObjectURL(blobUrl);
		}
	}

	start(track: MediaStreamTrack, onFinal: Finals): void {
		this.stop();
		const context = new AudioContext();
		const recognizer = new this.model.KaldiRecognizer(context.sampleRate);
		const began = performance.now();
		recognizer.on('partialresult', () => {
			if (this.spokenFrom === 0) this.spokenFrom = performance.now() - began;
		});
		recognizer.on('result', (message) => {
			const result = 'result' in message ? message.result : undefined;
			const text = result && 'text' in result ? String(result.text).trim() : '';
			const endMs = Math.round(performance.now() - began);
			const startMs = Math.min(endMs, Math.round(this.spokenFrom || endMs));
			this.spokenFrom = 0;
			if (text) onFinal({ text, startMs, endMs });
		});
		const processor = context.createScriptProcessor(4096, 1, 1);
		processor.onaudioprocess = (event) => {
			try {
				recognizer.acceptWaveform(event.inputBuffer);
			} catch {
				return;
			}
		};
		const source = context.createMediaStreamSource(new MediaStream([track]));
		source.connect(processor);
		processor.connect(context.destination);
		this.context = context;
		this.processor = processor;
		this.source = source;
		this.recognizer = recognizer;
	}

	stop(): void {
		const recognizer = this.recognizer;
		recognizer?.retrieveFinalResult();
		setTimeout(() => recognizer?.remove(), flushMs);
		this.source?.disconnect();
		this.processor?.disconnect();
		if (this.processor) this.processor.onaudioprocess = null;
		void this.context?.close();
		this.context = this.processor = this.source = this.recognizer = null;
		this.spokenFrom = 0;
	}

	dispose(): void {
		this.stop();
		this.model.terminate();
	}
}
