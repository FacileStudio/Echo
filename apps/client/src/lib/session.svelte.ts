import { fetchMe, type Me } from './api';
import { setSessionLocale } from './i18n/index.svelte';

export const session = $state<{ me: Me | null; loaded: boolean }>({ me: null, loaded: false });

/**
 * Asks the API who is signed in and adopts their Registre locale. A guest has no session: the
 * page keeps the locale cookie or the browser language.
 */
export async function loadSession(): Promise<Me | null> {
	session.me = await fetchMe();
	session.loaded = true;
	if (session.me?.locale) setSessionLocale(session.me.locale);
	return session.me;
}
