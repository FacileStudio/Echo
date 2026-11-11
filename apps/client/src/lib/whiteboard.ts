export const TOILE_BASE: string = import.meta.env.VITE_TOILE_BASE ?? 'https://toile.facile.studio';

export type BoardResult = { link: string } | { error: 'signIn' | 'failed' };

/**
 * The edit link in Toile's meeting-board answer (`share.url`, else `url`), resolved against
 * `base`. Anything that does not point at Toile's own origin is `null`.
 */
export function boardLink(body: unknown, base: string = TOILE_BASE): string | null {
	if (typeof body !== 'object' || body === null) return null;
	const { url, share } = body as { url?: unknown; share?: { url?: unknown } | null };
	const raw = typeof share?.url === 'string' ? share.url : url;
	if (typeof raw !== 'string' || raw === '') return null;
	try {
		const parsed = new URL(raw, base);
		return parsed.origin === new URL(base).origin ? parsed.href : null;
	} catch {
		return null;
	}
}

/** Asks Toile for the room's board (`POST /api/boards/meeting`, idempotent per room) with the Toile session cookie. */
export async function meetingBoard(room: string, fetcher: typeof fetch = fetch): Promise<BoardResult> {
	try {
		const res = await fetcher(`${TOILE_BASE}/api/boards/meeting`, {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'X-Facile-CSRF': '1' },
			body: JSON.stringify({ room })
		});
		if (res.status === 401) return { error: 'signIn' };
		if (!res.ok) return { error: 'failed' };
		const link = boardLink(await res.json().catch(() => null));
		return link ? { link } : { error: 'failed' };
	} catch {
		return { error: 'failed' };
	}
}

/**
 * Opens the room's board in a new tab. The tab opens before the request, inside the click, so
 * popup blockers let it through; it closes again when Toile refuses.
 */
export async function openWhiteboard(room: string): Promise<BoardResult> {
	const tab = window.open('', '_blank');
	const result = await meetingBoard(room);
	if (!('link' in result)) {
		tab?.close();
		return result;
	}
	if (tab) {
		tab.opener = null;
		tab.location.href = result.link;
	} else {
		window.open(result.link, '_blank', 'noopener');
	}
	return result;
}
