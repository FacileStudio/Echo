import COUNTRIES_EN from 'i18n-iso-countries/langs/en.json';
import COUNTRIES_FR from 'i18n-iso-countries/langs/fr.json';
import i18next from 'i18next';
import I18nextXHRBackend, { HttpBackendOptions } from 'i18next-http-backend';
import { merge } from 'lodash-es';

import LANGUAGES_RESOURCES from '../../../../lang/languages.json';
import MAIN_EN from '../../../../lang/main.json';
import MAIN_FR from '../../../../lang/main-fr.json';
import TRANSLATION_LANGUAGES_RESOURCES from '../../../../lang/translation-languages.json';

import { I18NEXT_INITIALIZED, LANGUAGE_CHANGED } from './actionTypes';
import languageDetector from './languageDetector';

const COUNTRIES_OVERRIDES = { countries: { TW: 'Taiwan' } };
const COUNTRIES_EN_MERGED = merge({}, COUNTRIES_EN, COUNTRIES_OVERRIDES);
const COUNTRIES_FR_MERGED = merge({}, COUNTRIES_FR, COUNTRIES_OVERRIDES);

/**
 * The available/supported languages.
 *
 * @public
 * @type {Array<string>}
 */
export const LANGUAGES: Array<string> = Object.keys(LANGUAGES_RESOURCES);

/**
 * The available/supported translation languages.
 *
 * @public
 * @type {Array<string>}
 */
export const TRANSLATION_LANGUAGES: Array<string> = Object.keys(TRANSLATION_LANGUAGES_RESOURCES);

export const DEFAULT_LANGUAGE = 'fr';

/**
 * The available/supported translation languages head. (Languages displayed on the top ).
 *
 * @public
 * @type {Array<string>}
 */
export const TRANSLATION_LANGUAGES_HEAD: Array<string> = [ DEFAULT_LANGUAGE ];

/**
 * The available/supported i18n namespaces.
 *
 * @public
 * @type {Array<string>}
 */
export const SUPPORTED_NS = [ 'main', 'languages', 'countries', 'translation-languages' ];

/**
 * The options to initialize i18next with.
 *
 * @type {i18next.InitOptions}
 */
const options: i18next.InitOptions = {
    backend: <HttpBackendOptions>{
        loadPath: (lng: string[], ns: string[]) => {
            switch (ns[0]) {
            case 'countries':
            case 'main':
                return 'lang/{{ns}}-{{lng}}.json';
            default:
                return 'lang/{{ns}}.json';
            }
        }
    },
    defaultNS: 'main',
    fallbackLng: DEFAULT_LANGUAGE,
    interpolation: {
        escapeValue: false // not needed for react as it escapes by default
    },
    load: 'all',
    ns: SUPPORTED_NS,
    react: {
        // re-render when a new resource bundle is added
        // @ts-expect-error. Fixed in i18next 19.6.1.
        bindI18nStore: 'added',
        useSuspense: false
    },
    returnEmptyString: false,
    returnNull: false,

    // XXX i18next modifies the array lngWhitelist so make sure to clone
    // LANGUAGES.
    whitelist: LANGUAGES.slice()
};

i18next
    .use(navigator.product === 'ReactNative' ? {} : I18nextXHRBackend)
    .use(languageDetector)
    .init(options);

// Pre-bundle French (default) and English so neither triggers HTTP 404s.
for (const [ lng, main, countries ] of [
    [ 'fr', MAIN_FR, COUNTRIES_FR_MERGED ],
    [ 'en', MAIN_EN, COUNTRIES_EN_MERGED ]
] as const) {
    i18next.addResourceBundle(lng, 'main', main, true, true);
    i18next.addResourceBundle(lng, 'countries', countries, true, true);
    i18next.addResourceBundle(lng, 'languages', LANGUAGES_RESOURCES, true, true);
    i18next.addResourceBundle(lng, 'translation-languages', TRANSLATION_LANGUAGES_RESOURCES, true, true);
}

// Add builtin languages.
// XXX: Note we are using require here, because we want the side-effects of the
// import, but imports can only be placed at the top, and it would be too early,
// since i18next is not yet initialized at that point.
require('./BuiltinLanguages');

// Label change through dynamic branding is available only for web
if (typeof APP !== 'undefined') {
    i18next.on('initialized', () => {
        APP.store.dispatch({ type: I18NEXT_INITIALIZED });
    });

    i18next.on('languageChanged', () => {
        APP.store.dispatch({ type: LANGUAGE_CHANGED });
    });
}

export default i18next;
