export interface Answer {
	transcribe: boolean | null;
	record: boolean | null;
}

export interface Person extends Answer {
	identity: string;
	name: string;
}

export interface CaptureState {
	transcription: boolean;
	recording: boolean;
	owner: boolean;
	you: Answer;
	people?: Person[];
}

/** A person's audio may be transcribed only when the organizer asked and that person said yes. */
export function mayTranscribe(state: CaptureState | null): boolean {
	return !!state && state.transcription && state.you.transcribe === true;
}

/** What this person has not answered yet: the banner asks only for those. */
export function pendingQuestions(state: CaptureState | null): { transcribe: boolean; record: boolean } {
	if (!state) return { transcribe: false, record: false };
	return {
		transcribe: state.transcription && state.you.transcribe === null,
		record: state.recording && state.you.record === null
	};
}

/**
 * The identities whose audio the organizer's recorder may mix: connected right now and recorded
 * "yes" while recording is on. Anyone who declined, was never asked, or just joined is left out,
 * so a recording never holds a voice that did not agree to it.
 */
export function recordable(state: CaptureState | null, present: readonly string[]): string[] {
	if (!state || !state.recording || !state.people) return [];
	const here = new Set(present);
	return state.people.filter((p) => p.record === true && here.has(p.identity)).map((p) => p.identity);
}

/** True when someone connected has not agreed to be recorded, so the organizer can be told. */
export function leftOut(state: CaptureState | null, present: readonly string[]): number {
	if (!state || !state.recording || !state.people) return 0;
	const included = new Set(recordable(state, present));
	return present.filter((identity) => !included.has(identity) && state.people!.some((p) => p.identity === identity)).length;
}
