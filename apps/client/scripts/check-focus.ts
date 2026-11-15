import assert from 'node:assert/strict';
import { nextPin, resolveFocus, togglePin, type Presence } from '../src/lib/focus';

const me: Presence = { identity: 'me', isLocal: true, sharing: false };
const ann: Presence = { identity: 'ann', isLocal: false, sharing: false };
const bob: Presence = { identity: 'bob', isLocal: false, sharing: true };
const mine: Presence = { identity: 'me', isLocal: true, sharing: true };
const annCam = { identity: 'ann', source: 'camera' } as const;
const bobScreen = { identity: 'bob', source: 'screen' } as const;

assert.equal(resolveFocus([me, ann], null), null, 'no share and no pin shows the grid');
assert.deepEqual(resolveFocus([me, ann, bob], null), bobScreen, 'a screen share takes the stage');
assert.deepEqual(resolveFocus([mine, bob], null), bobScreen, 'a remote share beats your own');
assert.deepEqual(resolveFocus([me, ann, bob], annCam), annCam, 'a pin beats the screen share');
assert.deepEqual(resolveFocus([me, bob], annCam), bobScreen, 'a pin on someone who left falls back');
assert.equal(resolveFocus([me, ann], { identity: 'ann', source: 'screen' }), null, 'a pinned share that stopped falls back');
assert.equal(resolveFocus([me, ann, bob], 'grid'), null, 'grid hides the spotlight');

assert.deepEqual(togglePin([me, ann], null, annCam), annCam, 'clicking a tile pins it');
assert.equal(togglePin([me, ann], annCam, annCam), 'grid', 'clicking the pinned tile unpins it');
assert.equal(togglePin([me, bob], null, bobScreen), 'grid', 'clicking the auto share returns to the grid');
assert.deepEqual(togglePin([me, ann, bob], null, annCam), annCam, 'clicking another tile moves the focus');

assert.equal(nextPin(['bob'], ['bob', 'ann'], 'grid'), null, 'a new share clears the pin');
assert.equal(nextPin(['bob'], ['bob'], 'grid'), 'grid', 'the same shares keep the pin');
assert.deepEqual(nextPin(['bob', 'ann'], ['bob'], annCam), annCam, 'a share that stops keeps the pin');

console.log('focus: ok');
