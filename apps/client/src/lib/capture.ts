import { API_BASE, ApiError, apiErrorMessage } from './api';
import { planLimitFrom } from './planLimit';
import type { CaptureState } from './consent';

export * from './consent';

export interface Utterance {
	text: string;
	start_ms: number;
	end_ms: number;
}

export function headers(seatKey: string, json = false): Record<string, string> {
	const out: Record<string, string> = { 'X-Facile-CSRF': '1' };
	if (json) out['Content-Type'] = 'application/json';
	if (seatKey) out['X-Echo-Seat-Key'] = seatKey;
	return out;
}

export const base = (slug: string) => `${API_BASE}/api/rooms/${encodeURIComponent(slug)}/capture`;

export async function expectOk(res: Response): Promise<void> {
	if (res.status === 402) throw await planLimitFrom(res);
	if (!res.ok) throw new ApiError(await apiErrorMessage(res), res.status);
}

export async function fetchCapture(slug: string, seatKey: string): Promise<CaptureState | null> {
	const res = await fetch(base(slug), { credentials: 'include', headers: headers(seatKey) });
	if (!res.ok) return null;
	const state = (await res.json()) as CaptureState;
	return { ...state, you: { transcribe: state.you?.transcribe ?? null, record: state.you?.record ?? null } };
}

export async function setCapture(
	slug: string,
	seatKey: string,
	flags: { transcription?: boolean; recording?: boolean }
): Promise<void> {
	await expectOk(
		await fetch(base(slug), {
			method: 'PUT',
			credentials: 'include',
			headers: headers(seatKey, true),
			body: JSON.stringify(flags)
		})
	);
}

export async function answerCapture(
	slug: string,
	seatKey: string,
	answer: { transcribe?: boolean; record?: boolean }
): Promise<void> {
	await expectOk(
		await fetch(`${base(slug)}/consent`, {
			method: 'PUT',
			credentials: 'include',
			headers: headers(seatKey, true),
			body: JSON.stringify(answer)
		})
	);
}

/** Sends one final line. The server decides who spoke from the session or seat key, never from here. */
export async function postUtterance(slug: string, seatKey: string, line: Utterance): Promise<void> {
	await expectOk(
		await fetch(`${API_BASE}/api/rooms/${encodeURIComponent(slug)}/transcript`, {
			method: 'POST',
			credentials: 'include',
			headers: headers(seatKey, true),
			body: JSON.stringify(line)
		})
	);
}
