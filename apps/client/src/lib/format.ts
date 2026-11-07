import { locale, t } from './i18n/index.svelte';

function parse(iso: string | undefined): Date | null {
	if (!iso) return null;
	const d = new Date(iso);
	return Number.isNaN(d.getTime()) ? null : d;
}

export function formatDateTime(iso: string | undefined): string {
	const d = parse(iso);
	return d ? new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short' }).format(d) : '—';
}

export function formatTime(iso: string | number | undefined): string {
	const d = typeof iso === 'number' ? new Date(iso) : parse(iso);
	return d ? new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }).format(d) : '—';
}

export function formatDuration(startedAt: string, endedAt?: string): string {
	const start = parse(startedAt);
	const end = parse(endedAt);
	if (!start || !end) return t('format.inProgress');
	const seconds = Math.max(0, Math.round((end.getTime() - start.getTime()) / 1000));
	const hours = Math.floor(seconds / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);
	if (hours > 0) return `${hours} h ${String(minutes).padStart(2, '0')}`;
	if (minutes > 0) return `${minutes} min ${String(seconds % 60).padStart(2, '0')} s`;
	return `${seconds} s`;
}
