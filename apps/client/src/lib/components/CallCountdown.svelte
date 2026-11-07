<script lang="ts">
	import { Alert, Badge } from '@facile/muse';
	import { fetchCallDeadline } from '$lib/deadline';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		deadline: string;
		slug: string;
		deadlineKey: string;
	}

	let { deadline, slug, deadlineKey }: Props = $props();

	const warnBeforeMs = 5 * 60 * 1000;
	const refreshMs = 60 * 1000;

	let now = $state(Date.now());
	let fetched = $state<string | null>(null);
	const current = $derived(fetched ?? deadline);
	const endsAt = $derived(current ? Date.parse(current) : NaN);
	const remaining = $derived(Number.isFinite(endsAt) ? Math.max(0, endsAt - now) : null);
	const label = $derived(remaining === null ? '' : clock(remaining));

	$effect(() => {
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
	});

	$effect(() => {
		if (!slug || !deadlineKey) return;
		const room = slug;
		const key = deadlineKey;
		const refresh = setInterval(async () => {
			const latest = await fetchCallDeadline(room, key);
			if (latest !== null) fetched = latest;
		}, refreshMs);
		return () => clearInterval(refresh);
	});

	function clock(ms: number): string {
		const total = Math.floor(ms / 1000);
		const minutes = Math.floor(total / 60);
		const seconds = String(total % 60).padStart(2, '0');
		return `${minutes}:${seconds}`;
	}
</script>

{#if remaining !== null}
	<div class="flex flex-col items-center gap-1 px-4 pb-3">
		<Badge tone={remaining <= warnBeforeMs ? 'warning' : 'neutral'}>{t('countdown.endsIn', { time: label })}</Badge>
		{#if remaining <= warnBeforeMs}
			<Alert tone="warning" title={t('countdown.soonTitle')}>{t('countdown.soonBody', { time: label })}</Alert>
		{/if}
	</div>
{/if}
