<script lang="ts">
	import { Alert, Badge } from '@facile/muse';

	interface Props {
		deadline: string;
	}

	let { deadline }: Props = $props();

	const warnBeforeMs = 5 * 60 * 1000;

	let now = $state(Date.now());
	const endsAt = $derived(Date.parse(deadline));
	const remaining = $derived(Number.isFinite(endsAt) ? Math.max(0, endsAt - now) : null);
	const label = $derived(remaining === null ? '' : clock(remaining));

	$effect(() => {
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
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
		<Badge tone={remaining <= warnBeforeMs ? 'warning' : 'neutral'}>Call ends in {label}</Badge>
		{#if remaining <= warnBeforeMs}
			<Alert tone="warning" title="This call ends soon">
				The host's plan limits call length. The call closes for everyone in {label}.
			</Alert>
		{/if}
	</div>
{/if}
