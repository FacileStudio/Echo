export const API_BASE: string = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080';

export interface Me {
	id: number;
	email: string;
	name: string;
	is_admin: boolean;
}

export interface TokenGrant {
	token: string;
	url: string;
}

export async function fetchMe(): Promise<Me | null> {
	try {
		const res = await fetch(`${API_BASE}/api/auth/me`, { credentials: 'include' });
		if (!res.ok) return null;
		const data = (await res.json()) as { user?: Me };
		return data.user ?? null;
	} catch {
		return null;
	}
}

export async function requestToken(
	slug: string,
	displayName?: string
): Promise<TokenGrant> {
	const res = await fetch(`${API_BASE}/api/rooms/${encodeURIComponent(slug)}/token`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', 'X-Facile-CSRF': '1' },
		credentials: 'include',
		body: JSON.stringify(displayName ? { display_name: displayName } : {})
	});
	if (!res.ok) throw new Error(`token request failed (${res.status})`);
	return (await res.json()) as TokenGrant;
}

export class ApiError extends Error {
	constructor(
		message: string,
		public status: number
	) {
		super(message);
	}
}

async function apiErrorMessage(res: Response): Promise<string> {
	try {
		const body = (await res.json()) as { error?: { code?: string; message?: string } };
		return body.error?.message ?? `request failed (${res.status})`;
	} catch {
		return `request failed (${res.status})`;
	}
}

export async function startRecording(slug: string): Promise<{ egressId: string }> {
	const res = await fetch(`${API_BASE}/api/rooms/${encodeURIComponent(slug)}/record/start`, {
		method: 'POST',
		headers: { 'X-Facile-CSRF': '1' },
		credentials: 'include'
	});
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	return (await res.json()) as { egressId: string };
}

export async function stopRecording(slug: string): Promise<void> {
	const res = await fetch(`${API_BASE}/api/rooms/${encodeURIComponent(slug)}/record/stop`, {
		method: 'POST',
		headers: { 'X-Facile-CSRF': '1' },
		credentials: 'include'
	});
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
}

export type CallId = string | number;

export interface CallSummary {
	content: string;
	model: string;
	created_at: string;
}

export interface CallListItem {
	id: CallId;
	started_at: string;
	ended_at?: string;
	has_recording?: boolean;
}

export interface CallParticipant {
	identity: string;
	name: string;
	joined_at: string;
	left_at?: string;
}

export interface CallDetail extends CallListItem {
	transcript?: string;
	summary?: CallSummary;
	participants: CallParticipant[];
}

async function getJSON<T>(path: string): Promise<T> {
	const res = await fetch(`${API_BASE}${path}`, { credentials: 'include' });
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	return (await res.json()) as T;
}

export async function fetchRoomCalls(slug: string): Promise<CallListItem[]> {
	const calls = await getJSON<CallListItem[] | null>(
		`/api/rooms/${encodeURIComponent(slug)}/calls`
	);
	return calls ?? [];
}

export async function fetchCall(id: CallId): Promise<CallDetail> {
	const call = await getJSON<CallDetail>(`/api/calls/${encodeURIComponent(String(id))}`);
	return { ...call, participants: call.participants ?? [] };
}

export async function generateCallSummary(id: CallId): Promise<CallSummary> {
	const res = await fetch(`${API_BASE}/api/calls/${encodeURIComponent(String(id))}/summary`, {
		method: 'POST',
		headers: { 'X-Facile-CSRF': '1' },
		credentials: 'include'
	});
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	return (await res.json()) as CallSummary;
}

export function callRecordingUrl(id: CallId): string {
	return `${API_BASE}/api/calls/${encodeURIComponent(String(id))}/recording`;
}

export interface RecordingFile {
	blob: Blob;
	filename: string;
}

/** Fetch a call recording as a blob.
 *
 * A plain `<a href>` cannot be used here: when the file is not on this node the
 * API answers with a JSON error envelope and no `Content-Disposition`, and the
 * browser renders that envelope as a page, throwing the user out of the SPA.
 */
export async function fetchCallRecording(id: CallId): Promise<RecordingFile> {
	const res = await fetch(callRecordingUrl(id), { credentials: 'include' });
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	return { blob: await res.blob(), filename: filenameFrom(res, id) };
}

function filenameFrom(res: Response, id: CallId): string {
	const disposition = res.headers.get('Content-Disposition') ?? '';
	const match = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(disposition);
	return match ? decodeURIComponent(match[1]) : `call-${id}.mp4`;
}

export function slugify(name: string): string {
	return name
		.toLowerCase()
		.normalize('NFD')
		.replace(/[\u0300-\u036f]/g, '')
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '');
}
