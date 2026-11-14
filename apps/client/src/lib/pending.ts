import { API_BASE, apiError, getJSON } from './api';

export interface PendingRoom {
	slug: string;
	name: string;
	meetings: number;
}

export interface PendingAction {
	id: string;
	kind: string;
	client_id: string;
	summary: string;
	created_at: string;
	expires_at: string;
	state: 'ready' | 'changed' | 'missing';
	room?: PendingRoom;
}

/** Lists the actions MCP clients asked for that wait on the signed-in person. */
export async function fetchPending(): Promise<PendingAction[]> {
	const list = await getJSON<{ actions: PendingAction[] | null }>('/api/mcp/pending');
	return list.actions ?? [];
}

/** Confirms or dismisses one action, once. */
export async function answerPending(id: string, answer: 'confirm' | 'dismiss'): Promise<void> {
	const res = await fetch(`${API_BASE}/api/mcp/pending/${encodeURIComponent(id)}/${answer}`, {
		method: 'POST',
		headers: { 'X-Facile-CSRF': '1' },
		credentials: 'include'
	});
	if (!res.ok) throw await apiError(res);
}

/** Names the client that asked: Bulle, the host of a client URL, else its id. */
export function clientName(clientId: string): string {
	if (clientId === 'bulle') return 'Bulle';
	try {
		return new URL(clientId).host;
	} catch {
		return clientId || '?';
	}
}
