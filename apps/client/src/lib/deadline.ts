import { API_BASE } from './api';

/**
 * Reads the room's current call deadline, so an upgrade, a downgrade or a server restart
 * reaches a running countdown. Resolves to `''` when nothing limits the call, and to `null`
 * when the server could not be asked, in which case the caller keeps what it has.
 */
export async function fetchCallDeadline(slug: string): Promise<string | null> {
	try {
		const res = await fetch(`${API_BASE}/api/rooms/${encodeURIComponent(slug)}/deadline`, {
			credentials: 'include'
		});
		if (!res.ok) return null;
		const body = (await res.json()) as { call_deadline?: unknown };
		return typeof body.call_deadline === 'string' ? body.call_deadline : '';
	} catch {
		return null;
	}
}
