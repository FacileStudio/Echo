<script lang="ts">
	import { Avatar, Badge, Button, Table } from '@facile/muse';
	import type { CallListItem } from '$lib/api';
	import { formatDateTime, formatDuration } from '$lib/format';
	import { t } from '$lib/i18n/index.svelte';
	import type { Meeting } from '$lib/meetings';

	interface Props {
		calls: Array<CallListItem & Partial<Meeting>>;
		showMeeting?: boolean;
	}

	let { calls, showMeeting = false }: Props = $props();

	const shown = 5;

	function name(p: { name: string }): string {
		return p.name || t('participants.guest');
	}
</script>

<Table>
	<thead>
		<tr>
			<th scope="col">{t('history.started')}</th>
			{#if showMeeting}
				<th scope="col">{t('history.room')}</th>
				<th scope="col">{t('history.participants')}</th>
			{/if}
			<th scope="col">{t('history.duration')}</th>
			<th scope="col">{t('history.recording')}</th>
			{#if showMeeting}
				<th scope="col">{t('history.transcript')}</th>
			{/if}
			<th scope="col"><span class="sr-only">{t('history.details')}</span></th>
		</tr>
	</thead>
	<tbody>
		{#each calls as call (call.id)}
			<tr>
				<td class="whitespace-nowrap">{formatDateTime(call.started_at)}</td>
				{#if showMeeting}
					<td>
						<span class="flex flex-wrap items-center gap-1">
							<span class="font-medium text-fc-ink">{call.room_name}</span>
							{#if call.role === 'owner'}
								<Badge tone="owner">{t('history.owner')}</Badge>
							{/if}
						</span>
					</td>
					<td>
						<span class="flex items-center gap-1">
							{#each (call.participants ?? []).slice(0, shown) as p (p.identity)}
								<Avatar src={p.avatar_url} name={name(p)} size="sm" title={name(p)} class="rounded-full" />
							{/each}
							{#if (call.participants ?? []).length > shown}
								<span class="text-fc-subtext text-fc-muted">+{(call.participants ?? []).length - shown}</span>
							{/if}
						</span>
					</td>
				{/if}
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
						<span class="text-fc-muted">{t('history.none')}</span>
					{/if}
				</td>
				{#if showMeeting}
					<td class="whitespace-nowrap">
						{#if call.has_transcript}
							<Badge tone="info">{t('history.available')}</Badge>
						{:else}
							<span class="text-fc-muted">{t('history.none')}</span>
						{/if}
					</td>
				{/if}
				<td class="text-right">
					<Button href="/calls/{call.id}" variant="ghost" size="sm">{t('history.open')}</Button>
				</td>
			</tr>
		{/each}
	</tbody>
</Table>
