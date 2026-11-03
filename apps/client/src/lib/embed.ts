export interface EmbedGrant {
	token: string;
	url: string;
	key: string;
	name: string;
}

/**
 * Reads the join token, LiveKit URL and end-to-end encryption key an
 * integrating app put in the embed URL's fragment, which the browser never
 * sends to a server. The key may be empty; the caller refuses to join then.
 */
export function readEmbedGrant(hash: string): EmbedGrant | null {
	const params = new URLSearchParams(hash.replace(/^#/, ''));
	const token = params.get('token');
	const url = params.get('url');
	if (!token || !url) return null;
	return { token, url, key: params.get('key')?.trim() ?? '', name: tokenName(token) };
}

/** Returns the display name carried in a LiveKit token, or an empty string. */
export function tokenName(token: string): string {
	try {
		const payload = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/');
		const bytes = Uint8Array.from(atob(payload), (c) => c.charCodeAt(0));
		const claims = JSON.parse(new TextDecoder().decode(bytes)) as { name?: string };
		return claims.name ?? '';
	} catch {
		return '';
	}
}
