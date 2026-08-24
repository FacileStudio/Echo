<script lang="ts">
	import { Badge, Button, Table } from '@facile/muse';
	import type { CallListItem } from '$lib/api';
	import { formatDateTime, formatDuration } from '$lib/format';

	interface Props {
		calls: CallListItem[];
	}

	let { calls }: Props = $props();
</script>

<Table>
	<thead>
		<tr>
			<th scope="col">Started</th>
			<th scope="col">Duration</th>
			<th scope="col">Recording</th>
			<th scope="col"><span class="sr-only">Details</span></th>
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
						<Badge tone="success">In progress</Badge>
					{/if}
				</td>
				<td class="whitespace-nowrap">
					{#if call.recording_path}
						<Badge tone="info">Available</Badge>
					{:else}
						<span class="text-fc-fg-muted">None</span>
					{/if}
				</td>
				<td class="text-right">
					<Button href="/calls/{call.id}" variant="ghost" size="sm">Open</Button>
				</td>
			</tr>
		{/each}
	</tbody>
</Table>
