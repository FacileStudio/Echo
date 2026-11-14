import assert from 'node:assert/strict';
import { OWN_SESSION_REFUSAL, isOwnSessionRefusal } from '../src/lib/refusal';

const refusal = (status: number, code: string, message: string) => Object.assign(new Error(message), { status, code });

assert.equal(isOwnSessionRefusal(refusal(403, 'permission_denied', OWN_SESSION_REFUSAL)), true);
assert.equal(isOwnSessionRefusal(refusal(403, 'permission_denied', 'access denied')), false);
assert.equal(isOwnSessionRefusal(refusal(403, '', OWN_SESSION_REFUSAL)), false);
assert.equal(isOwnSessionRefusal(refusal(401, 'permission_denied', OWN_SESSION_REFUSAL)), false);
assert.equal(isOwnSessionRefusal(null), false);
console.log('own-session refusal: 5 cases agree');
