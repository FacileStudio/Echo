export interface EmbedGrant {
	token: string;
	url: string;
	name: string;
}

/**
 * Reads the join token and LiveKit URL an integrating app put in the embed
 * URL's fragment, which the browser never sends to a server.
 */
export function readEmbedGrant(hash: string): EmbedGrant | null {
	const params = new URLSearchParams(hash.replace(/^#/, ''));
	const token = params.get('token');
	const url = params.get('url');
	if (!token || !url) return null;
	return { token, url, name: tokenName(token) };
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
