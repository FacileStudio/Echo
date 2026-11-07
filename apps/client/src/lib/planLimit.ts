import { t } from './i18n/index.svelte';

type Envelope = Record<string, unknown>;

const isRecord = (value: unknown): value is Envelope => typeof value === 'object' && value !== null;

const text = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

/**
 * True for a same-origin path or an https URL with a host and no credentials, the rule the API
 * applies before it sends `upgrade_url`. Anything else never reaches an `href`.
 */
export function isSafeUpgradeUrl(raw: string): boolean {
	if (!raw || /[\s\\\p{Cc}]/u.test(raw)) return false;
	if (raw.startsWith('/')) return !raw.startsWith('//');
	try {
		const url = new URL(raw);
		return url.protocol === 'https:' && url.host !== '' && !url.username && !url.password;
	} catch {
		return false;
	}
}

/** The API's 402: the host's plan does not allow what was asked. */
export class PlanLimitError extends Error {
	constructor(
		message: string,
		public key: string,
		public limit: number | null,
		public upgradeUrl: string
	) {
		super(message);
	}

	/** Reads a 402 body in the suite error envelope, dropping an unsafe `upgrade_url`. */
	static from(body: unknown): PlanLimitError {
		const inner = isRecord(body) && isRecord(body.error) ? body.error : {};
		const upgrade = text(inner.upgrade_url);
		const limit = typeof inner.limit === 'number' && Number.isFinite(inner.limit) ? inner.limit : null;
		return new PlanLimitError(
			text(inner.message) || t('plan.default'),
			text(inner.key),
			limit,
			isSafeUpgradeUrl(upgrade) ? upgrade : ''
		);
	}
}

/** Reads a 402 response into a PlanLimitError, whatever its body holds. */
export async function planLimitFrom(res: Response): Promise<PlanLimitError> {
	try {
		return PlanLimitError.from(await res.json());
	} catch {
		return PlanLimitError.from(null);
	}
}

/** What the person joining is told, per refused feature. */
export function planLimitText(error: PlanLimitError): string {
	if (error.key === 'echo.max_participants' && error.limit !== null) return t('plan.full', { count: error.limit });
	if (error.key === 'echo.recording') return t('plan.noRecording');
	if (error.key === 'echo.max_call_minutes' && error.limit !== null) return t('plan.minutes', { count: error.limit });
	if (error.key === 'echo.max_call_minutes') return t('plan.minutesGeneric');
	return error.message;
}
