export type FocusSource = 'camera' | 'screen';

export interface Focus {
	identity: string;
	source: FocusSource;
}

export interface Presence {
	identity: string;
	isLocal: boolean;
	sharing: boolean;
}

export type Pin = Focus | 'grid' | null;

/** True when both focuses name the same participant and the same video. */
export function sameFocus(a: Focus | null, b: Focus | null): boolean {
	return !!a && !!b && a.identity === b.identity && a.source === b.source;
}

/** The first remote screen share, else the local one, else nothing. */
function autoFocus(people: Presence[]): Focus | null {
	const sharer = people.filter((p) => p.sharing).sort((a, b) => Number(a.isLocal) - Number(b.isLocal))[0];
	return sharer ? { identity: sharer.identity, source: 'screen' } : null;
}

/**
 * The video shown large on the stage. A pin wins while its participant and video are still
 * there, 'grid' shows no spotlight, and anything else falls back to the first screen share.
 */
export function resolveFocus(people: Presence[], pin: Pin): Focus | null {
	if (pin === 'grid') return null;
	if (pin) {
		const person = people.find((p) => p.identity === pin.identity);
		if (person && (pin.source === 'camera' || person.sharing)) return pin;
	}
	return autoFocus(people);
}

/** Clicking the video in focus returns to the grid; clicking any other video pins it. */
export function togglePin(people: Presence[], pin: Pin, target: Focus): Pin {
	return sameFocus(resolveFocus(people, pin), target) ? 'grid' : target;
}

/** A screen share that just started clears the pin, so the new share takes the stage. */
export function nextPin(before: string[], after: string[], pin: Pin): Pin {
	return after.some((id) => !before.includes(id)) ? null : pin;
}
