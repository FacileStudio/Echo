const dateTimeFormat = new Intl.DateTimeFormat('fr-FR', {
	dateStyle: 'medium',
	timeStyle: 'short'
});

const timeFormat = new Intl.DateTimeFormat('fr-FR', {
	hour: '2-digit',
	minute: '2-digit'
});

function parse(iso: string | undefined): Date | null {
	if (!iso) return null;
	const d = new Date(iso);
	return Number.isNaN(d.getTime()) ? null : d;
}

export function formatDateTime(iso: string | undefined): string {
	const d = parse(iso);
	return d ? dateTimeFormat.format(d) : '—';
}

export function formatTime(iso: string | undefined): string {
	const d = parse(iso);
	return d ? timeFormat.format(d) : '—';
}

export function formatDuration(startedAt: string, endedAt?: string): string {
	const start = parse(startedAt);
	const end = parse(endedAt);
	if (!start || !end) return 'In progress';
	const seconds = Math.max(0, Math.round((end.getTime() - start.getTime()) / 1000));
	const hours = Math.floor(seconds / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);
	if (hours > 0) return `${hours} h ${String(minutes).padStart(2, '0')}`;
	if (minutes > 0) return `${minutes} min ${String(seconds % 60).padStart(2, '0')} s`;
	return `${seconds} s`;
}
