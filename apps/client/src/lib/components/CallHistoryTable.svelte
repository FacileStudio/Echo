<script lang="ts">
	import { Badge, Button, Table } from '@facile/muse';
	import type { CallListItem } from '$lib/api';
	import { formatDateTime, formatDuration } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		calls: CallListItem[];
	}

	let { calls }: Props = $props();
</script>

<Table>
	<thead>
		<tr>
			<th scope="col">{t('history.started')}</th>
			<th scope="col">{t('history.duration')}</th>
			<th scope="col">{t('history.recording')}</th>
			<th scope="col"><span class="sr-only">{t('history.details')}</span></th>
		</tr>
	</thead>
	<tbody>
		{#each calls as call (call.id)}
			<tr>
				<td class="whitespace-nowrap">{formatDateTime(call.started_at)}</td>
				<td class="whitespace-nowrap">
					{#if call.ended_at}
						{formatDuration(call.started_at, call.ended_at)}
					{:else}
						<Badge tone="success">{t('history.inProgress')}</Badge>
					{/if}
				</td>
				<td class="whitespace-nowrap">
					{#if call.has_recording}
						<Badge tone="info">{t('history.available')}</Badge>
					{:else}
						<span class="text-fc-fg-muted">{t('history.none')}</span>
					{/if}
				</td>
				<td class="text-right">
					<Button href="/calls/{call.id}" variant="ghost" size="sm">{t('history.open')}</Button>
				</td>
			</tr>
		{/each}
	</tbody>
</Table>
