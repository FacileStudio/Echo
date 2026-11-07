<script lang="ts">
	import { Alert, Button } from '@facile/muse';
	import { t } from '$lib/i18n/index.svelte';
	import { planLimitText, type PlanLimitError } from '$lib/planLimit';

	interface Props {
		error: PlanLimitError;
	}

	let { error }: Props = $props();
</script>

<div class="flex flex-col gap-1">
	<Alert tone="warning" title={t('plan.title')}>{planLimitText(error)}</Alert>
	{#if error.upgradeUrl}
		<Button href={error.upgradeUrl} target="_blank" rel="noopener noreferrer" size="lg" class="w-full">
			{t('plan.upgrade')}
		</Button>
	{/if}
</div>
