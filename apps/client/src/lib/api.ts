import { t } from './i18n/index.svelte';
import { planLimitFrom } from './planLimit';

export const API_BASE: string = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080';

export interface Me {
	id: number;
	email: string;
	name: string;
	picture?: string;
	locale: string;
	is_admin: boolean;
}

export interface TokenGrant {
	token: string;
	url: string;
	call_deadline?: string;
	deadline_key?: string;
	seat_key?: string;
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
	if (res.status === 402) throw await planLimitFrom(res);
	if (res.status === 429) throw new Error(t('room.tooMany'));
	if (!res.ok) throw new Error(t('room.tokenFailed', { status: res.status }));
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

export async function apiErrorMessage(res: Response): Promise<string> {
	try {
		const body = (await res.json()) as { error?: unknown };
		const message = typeof body.error === 'string' ? body.error : (body.error as { message?: unknown } | undefined)?.message;
		return typeof message === 'string' && message ? message : t('api.failed', { status: res.status });
	} catch {
		return t('api.failed', { status: res.status });
	}
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
	avatar_url?: string;
	joined_at: string;
	left_at?: string;
}

export interface CallDetail extends CallListItem {
	room_slug: string;
	room_name: string;
	role: 'owner' | 'participant';
	has_transcript: boolean;
	summary?: CallSummary;
	participants: CallParticipant[];
}

export async function getJSON<T>(path: string): Promise<T> {
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
	if (res.status === 402) throw await planLimitFrom(res);
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
	return (await res.json()) as CallSummary;
}

export function slugify(name: string): string {
	return name
		.toLowerCase()
		.normalize('NFD')
		.replace(/[\u0300-\u036f]/g, '')
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '');
}
