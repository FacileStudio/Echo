<script lang="ts">
	import { Table } from '@facile/muse';
	import type { CallParticipant } from '$lib/api';
	import { formatTime } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		participants: CallParticipant[];
	}

	let { participants }: Props = $props();
</script>

{#if participants.length === 0}
	<p class="text-fc-p text-fc-muted">{t('participants.none')}</p>
{:else}
	<Table>
		<thead>
			<tr>
				<th scope="col">{t('participants.participant')}</th>
				<th scope="col">{t('participants.joined')}</th>
				<th scope="col">{t('participants.left')}</th>
			</tr>
		</thead>
		<tbody>
			{#each participants as p (p.identity + p.joined_at)}
				<tr>
					<td>
						<span class="font-medium text-fc-ink">{p.name || p.identity}</span>
					</td>
					<td class="whitespace-nowrap">{formatTime(p.joined_at)}</td>
					<td class="whitespace-nowrap">
						{#if p.left_at}
							{formatTime(p.left_at)}
						{:else}
							<span class="text-fc-muted">{t('participants.still')}</span>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</Table>
{/if}
