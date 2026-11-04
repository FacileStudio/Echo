<script lang="ts">
	import { Alert, Button } from '@facile/muse';
	import { planLimitText, type PlanLimitError } from '$lib/planLimit';

	interface Props {
		error: PlanLimitError;
	}

	let { error }: Props = $props();
</script>

<div class="flex flex-col gap-1">
	<Alert tone="warning" title="Plan limit reached">{planLimitText(error)}</Alert>
	{#if error.upgradeUrl}
		<Button href={error.upgradeUrl} target="_blank" rel="noopener noreferrer" size="lg" class="w-full">
			Upgrade the plan
		</Button>
	{/if}
</div>
