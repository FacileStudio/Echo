import { API_BASE, ApiError, apiErrorMessage, type CallId } from './api';
import { planLimitFrom } from './planLimit';

export interface TranscriptLine {
	speaker_id: string;
	speaker: string;
	text: string;
	offset_ms?: number;
}

export interface Transcript {
	total: number;
	lines: TranscriptLine[];
}

export type TranscriptFormat = 'txt' | 'vtt';

function transcriptPath(callId: CallId, query: string): string {
	return `/api/calls/${encodeURIComponent(String(callId))}/transcript${query}`;
}

export async function fetchTranscript(callId: CallId, search: string): Promise<Transcript> {
	const query = search.trim() ? `?q=${encodeURIComponent(search.trim())}` : '';
	const res = await fetch(`${API_BASE}${transcriptPath(callId, query)}`, { credentials: 'include' });
	if (res.status === 402) throw await planLimitFrom(res);
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	const result = (await res.json()) as Transcript;
	return { ...result, lines: result.lines ?? [] };
}

export async function downloadTranscript(callId: CallId, format: TranscriptFormat): Promise<void> {
	const res = await fetch(`${API_BASE}${transcriptPath(callId, `?format=${format}`)}`, { credentials: 'include' });
	if (res.status === 402) throw await planLimitFrom(res);
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	saveBlob(await res.blob(), `echo-${callId}.${format}`);
}

/** Hands a blob to the browser as a download. The URL is revoked on the next turn: sooner cancels it in Chrome. */
export function saveBlob(blob: Blob, filename: string): void {
	const objectUrl = URL.createObjectURL(blob);
	const anchor = document.createElement('a');
	anchor.href = objectUrl;
	anchor.download = filename;
	anchor.click();
	setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
}
