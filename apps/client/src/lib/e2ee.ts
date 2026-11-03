import { ExternalE2EEKeyProvider, isE2EESupported, type Room } from 'livekit-client';

export { isE2EESupported };

const ownKeyError =
	"Can't decrypt this call: your link is missing the right key. Ask for the full link, including everything after #.";

/**
 * Words a decrypt failure for the person who sees it. When nobody else in the call can be
 * decrypted, this browser holds the wrong key; otherwise the named participants do.
 */
export function decryptError(failing: string[], remotes: number): string {
	if (failing.length === 0 || failing.length >= remotes) return ownKeyError;
	return `Can't decrypt ${failing.join(', ')}: they joined with a different key. Send them the full link.`;
}

/** Returns 32 random bytes as base64url, the format of an Echo room key. */
export function newRoomKey(): string {
	const bytes = crypto.getRandomValues(new Uint8Array(32));
	return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/** Reads the `key` parameter of a URL fragment, or an empty string. */
export function readKey(hash: string): string {
	return new URLSearchParams(hash.replace(/^#/, '')).get('key')?.trim() ?? '';
}

const storageName = (slug: string) => `echo.key.${slug}`;

function remembered(slug: string): string {
	try {
		return localStorage.getItem(storageName(slug)) ?? '';
	} catch {
		return '';
	}
}

function remember(slug: string, key: string) {
	try {
		localStorage.setItem(storageName(slug), key);
	} catch {
		return;
	}
}

/**
 * Resolves the key of an Echo room: the one in the link, else the one this browser used last for
 * the slug, else a fresh one. The key is written back into the fragment so that copying the
 * address shares it, and it never reaches a server.
 */
export function roomKey(slug: string): string {
	const key = readKey(location.hash) || remembered(slug) || newRoomKey();
	const params = new URLSearchParams(location.hash.replace(/^#/, ''));
	params.set('key', key);
	remember(slug, key);
	history.replaceState(history.state, '', `${location.pathname}${location.search}#${params}`);
	return key;
}

/** Builds the encryption options of a LiveKit room: a shared-key provider and its worker. */
export function encryption() {
	return {
		keyProvider: new ExternalE2EEKeyProvider(),
		worker: new Worker(new URL('livekit-client/e2ee-worker', import.meta.url), { type: 'module' })
	};
}

/**
 * Sets the shared key, turns encryption on, then connects: media and data leave this browser
 * encrypted with a key the media server never sees.
 */
export async function connectEncrypted(
	room: Room,
	keyProvider: ExternalE2EEKeyProvider,
	key: string,
	target: { url: string; token: string }
) {
	await keyProvider.setKey(key);
	await room.setE2EEEnabled(true);
	await room.connect(target.url, target.token);
}
