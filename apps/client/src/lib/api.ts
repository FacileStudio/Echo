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

export function slugify(name: string): string {
	return name
		.toLowerCase()
		.normalize('NFD')
		.replace(/[\u0300-\u036f]/g, '')
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '');
}
