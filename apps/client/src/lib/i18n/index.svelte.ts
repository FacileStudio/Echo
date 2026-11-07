import { createTranslator, LOCALE_COOKIE, resolveLocale, type Locale } from '@facile/muse';
import { catalogs } from './messages';

const session = $state<{ locale: string | null }>({ locale: null });
let active: () => Locale = () => 'en';

/** Records the locale the signed-in person has on their Registre profile. */
export function setSessionLocale(locale: string | null) {
	session.locale = locale;
}

function cookieLocale(): string | null {
	if (typeof document === 'undefined') return null;
	const prefix = `${LOCALE_COOKIE}=`;
	const found = document.cookie.split('; ').find((part) => part.startsWith(prefix));
	return found ? decodeURIComponent(found.slice(prefix.length)) : null;
}

/** The locale source for `provideLocale`: session, then the picked cookie, then the browser. */
export function localeSource(): string | null {
	return session.locale ?? cookieLocale() ?? (typeof navigator === 'undefined' ? null : navigator.language);
}

/** Points the translator and the Intl helpers at the locale `provideLocale` resolved. */
export function bindLocale(source: () => Locale) {
	active = source;
}

/** The locale currently in force. */
export function locale(): Locale {
	return resolveLocale(active());
}

/** Translates an app message in the current locale. */
export const t = createTranslator(catalogs, () => active());
