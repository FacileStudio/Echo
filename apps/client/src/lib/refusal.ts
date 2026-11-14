export const OWN_SESSION_REFUSAL = "only the person's own session can confirm actions";

/**
 * Reports whether err is porte's refusal of a session that is not the person's own
 * browser session: a cookie from before the upgrade, a CLI or API token, or the mobile
 * or desktop app. Signing in again fixes only this one; any other 403 is the confirmed
 * action's own refusal.
 */
export function isOwnSessionRefusal(err: unknown): boolean {
	const e = err as { status?: unknown; code?: unknown; message?: unknown } | null;
	return e?.status === 403 && e.code === 'permission_denied' && e.message === OWN_SESSION_REFUSAL;
}
