import assert from 'node:assert/strict';
import { leftOut, mayTranscribe, pendingQuestions, recordable, type CaptureState, type Person } from '../src/lib/consent';

const person = (identity: string, record: boolean | null, transcribe: boolean | null = null): Person => ({
	identity,
	name: identity,
	record,
	transcribe
});

const owner = (people: Person[], flags: Partial<CaptureState> = {}): CaptureState => ({
	transcription: false,
	recording: true,
	owner: true,
	you: { transcribe: null, record: true },
	people,
	...flags
});

const guest = (flags: Partial<CaptureState>): CaptureState => ({
	transcription: false,
	recording: false,
	owner: false,
	you: { transcribe: null, record: null },
	...flags
});

const people = [person('user-1', true), person('guest-a', true), person('guest-b', false), person('guest-c', null)];
const present = ['user-1', 'guest-a', 'guest-b', 'guest-c', 'guest-gone'];

assert.deepEqual(recordable(owner(people), present), ['user-1', 'guest-a'], 'only the voices that said yes are mixed');
assert.deepEqual(recordable(owner(people), ['user-1']), ['user-1'], 'someone who left is not mixed');
assert.deepEqual(recordable(owner(people, { recording: false }), present), [], 'nothing is mixed while recording is off');
assert.deepEqual(recordable(guest({ recording: true }), present), [], 'a participant never builds a mix');
assert.deepEqual(recordable(null, present), [], 'no state, no mix');
assert.equal(leftOut(owner(people), present), 2, 'declined and unanswered people are counted as left out');
assert.equal(leftOut(owner(people, { recording: false }), present), 0);

assert.equal(mayTranscribe(guest({ transcription: true, you: { transcribe: true, record: null } })), true);
assert.equal(mayTranscribe(guest({ transcription: true, you: { transcribe: false, record: null } })), false, 'declined');
assert.equal(mayTranscribe(guest({ transcription: true })), false, 'never answered counts as no');
assert.equal(mayTranscribe(guest({ transcription: false, you: { transcribe: true, record: null } })), false, 'switched off');
assert.equal(mayTranscribe(null), false);

assert.deepEqual(pendingQuestions(guest({ transcription: true, recording: true })), { transcribe: true, record: true });
assert.deepEqual(
	pendingQuestions(guest({ transcription: true, recording: true, you: { transcribe: true, record: null } })),
	{ transcribe: false, record: true },
	'a later request is asked on its own'
);
assert.deepEqual(pendingQuestions(guest({})), { transcribe: false, record: false });

console.log('capture consent logic ok');
