import assert from 'node:assert/strict';
import { boardLink, meetingBoard } from '../src/lib/whiteboard';

const base = 'https://toile.facile.studio';

assert.equal(boardLink({ share: { url: `${base}/s/abc` } }, base), `${base}/s/abc`, 'the share link wins');
assert.equal(boardLink({ url: '/s/abc' }, base), `${base}/s/abc`, 'a relative link resolves against Toile');
assert.equal(boardLink({ url: 'https://evil.example/s/abc' }, base), null, 'another origin is refused');
assert.equal(boardLink({ url: 'javascript:alert(1)' }, base), null, 'a script link is refused');
assert.equal(boardLink({ share: null }, base), null);
assert.equal(boardLink(null, base), null);

const answer = (status: number, body: unknown): typeof fetch =>
	(async () => new Response(JSON.stringify(body), { status })) as unknown as typeof fetch;

let sent: RequestInit | undefined;
const spy = (async (_url: string, init: RequestInit) => {
	sent = init;
	return new Response(JSON.stringify({ share: { url: `${base}/s/x` } }), { status: 200 });
}) as unknown as typeof fetch;

assert.deepEqual(await meetingBoard('daily', spy), { link: `${base}/s/x` });
assert.equal(sent?.body, JSON.stringify({ room: 'daily' }));
assert.equal(new Headers(sent?.headers).get('X-Facile-CSRF'), '1', 'the cookie mutation carries the CSRF header');
assert.equal(sent?.credentials, 'include');
assert.deepEqual(await meetingBoard('daily', answer(401, {})), { error: 'signIn' });
assert.deepEqual(await meetingBoard('daily', answer(500, {})), { error: 'failed' });
assert.deepEqual(await meetingBoard('daily', answer(200, { url: 'https://evil.example' })), { error: 'failed' });
assert.deepEqual(
	await meetingBoard('daily', (async () => Promise.reject(new Error('offline'))) as unknown as typeof fetch),
	{ error: 'failed' }
);

console.log('whiteboard: ok');
