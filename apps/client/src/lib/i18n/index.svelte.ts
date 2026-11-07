import { catalogs, type Locale, type Message, type MessageKey } from './messages';

const supported: readonly Locale[] = ['fr', 'en', 'es', 'de'];
const session = $state<{ locale: string | null }>({ locale: null });

/** Records the locale the signed-in person has on their Registre profile. */
export function setSessionLocale(locale: string | null) {
	session.locale = locale;
}

function resolve(tag: string | null | undefined): Locale {
	const base = (tag ?? '').slice(0, 2).toLowerCase();
	return supported.find((candidate) => candidate === base) ?? 'en';
}

/** The locale in force: the Registre profile, else the browser language, else English. */
export function locale(): Locale {
	return resolve(session.locale ?? (typeof navigator === 'undefined' ? null : navigator.language));
}

function pick(message: Message, params: Record<string, string | number> | undefined, current: Locale): string {
	let text = message;
	if (typeof text !== 'string') {
		const category = new Intl.PluralRules(current).select(Number(params?.count ?? 0));
		text = category === 'one' ? text.one : text.other;
	}
	return text.replace(/\{(\w+)\}/g, (whole, name: string) => String(params?.[name] ?? whole));
}

/** Translates an app message in the current locale. */
export function t(key: MessageKey, params?: Record<string, string | number>): string {
	const current = locale();
	return pick(catalogs[current][key], params, current);
}
