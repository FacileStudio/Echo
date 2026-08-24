<script lang="ts">
	import { Table } from '@facile/muse';
	import type { CallParticipant } from '$lib/api';
	import { formatTime } from '$lib/format';

	interface Props {
		participants: CallParticipant[];
	}

	let { participants }: Props = $props();
</script>

{#if participants.length === 0}
	<p class="text-fc-sm text-fc-fg-muted">No participants recorded for this call.</p>
{:else}
	<Table>
		<thead>
			<tr>
				<th scope="col">Participant</th>
				<th scope="col">Joined</th>
				<th scope="col">Left</th>
			</tr>
		</thead>
		<tbody>
			{#each participants as p (p.identity + p.joined_at)}
				<tr>
					<td>
						<span class="font-medium text-fc-fg">{p.name || p.identity}</span>
					</td>
					<td class="whitespace-nowrap">{formatTime(p.joined_at)}</td>
					<td class="whitespace-nowrap">
						{#if p.left_at}
							{formatTime(p.left_at)}
						{:else}
							<span class="text-fc-fg-muted">Still connected</span>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</Table>
{/if}
