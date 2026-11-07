import { API_BASE, ApiError, apiErrorMessage, getJSON, type CallId, type CallParticipant } from './api';
import { planLimitFrom } from './planLimit';

export interface Meeting {
	id: CallId;
	room_slug: string;
	room_name: string;
	role: 'owner' | 'participant';
	started_at: string;
	ended_at?: string;
	duration_seconds?: number;
	has_recording: boolean;
	has_transcript: boolean;
	participants: CallParticipant[];
}

export interface MeetingsPage {
	items: Meeting[];
	page: number;
	page_size: number;
	total: number;
}

export interface Recording {
	id: string;
	created_at: string;
	shared: boolean;
	can_share: boolean;
}

export async function fetchMeetings(page: number): Promise<MeetingsPage> {
	const result = await getJSON<MeetingsPage>(`/api/meetings?page=${page}`);
	return { ...result, items: (result.items ?? []).map((m) => ({ ...m, participants: m.participants ?? [] })) };
}

export async function fetchRecordings(callId: CallId): Promise<Recording[]> {
	return (await getJSON<Recording[] | null>(`/api/calls/${encodeURIComponent(String(callId))}/recordings`)) ?? [];
}

export function recordingUrl(id: string, download = false): string {
	return `${API_BASE}/api/recordings/${encodeURIComponent(id)}/file${download ? '?download=1' : ''}`;
}

/** Asks for the first byte, so a recording missing on this node is reported here and not as a JSON page. */
export async function probeRecording(id: string): Promise<void> {
	const res = await fetch(recordingUrl(id), { credentials: 'include', headers: { Range: 'bytes=0-0' } });
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	await res.body?.cancel();
}

export async function setRecordingShared(id: string, shared: boolean): Promise<Recording> {
	const res = await fetch(`${API_BASE}/api/recordings/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', 'X-Facile-CSRF': '1' },
		credentials: 'include',
		body: JSON.stringify({ shared })
	});
	if (res.status === 402) throw await planLimitFrom(res);
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	return (await res.json()) as Recording;
}
