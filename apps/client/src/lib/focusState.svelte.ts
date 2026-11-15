import { untrack } from 'svelte';
import { nextPin, resolveFocus, togglePin, type Focus, type Pin, type Presence } from './focus';

/** The stage's spotlight for one call: what the user pinned, and what that resolves to. */
export class FocusState {
	pin = $state<Pin>(null);
	#sharers: string[] = [];
	#presence: () => Presence[];

	constructor(presence: () => Presence[]) {
		this.#presence = presence;
	}

	get focus(): Focus | null {
		return resolveFocus(this.#presence(), this.pin);
	}

	/** Clears the pin when a new screen share starts; call it from an effect. */
	sync() {
		const now = this.#presence().filter((p) => p.sharing).map((p) => p.identity);
		const before = this.#sharers;
		this.#sharers = now;
		this.pin = nextPin(before, now, untrack(() => this.pin));
	}

	select(target: Focus) {
		this.pin = togglePin(this.#presence(), this.pin, target);
	}

	unpin() {
		this.pin = 'grid';
	}
}
