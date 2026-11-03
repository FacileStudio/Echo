<script lang="ts">
	import type { LocalParticipant, RemoteParticipant } from 'livekit-client';
	import { gridCols, gridColumn, gridRows, tileWidth } from '$lib/grid';
	import { isSharing } from '$lib/screenShare';
	import ParticipantTile from './ParticipantTile.svelte';

	type Participant = LocalParticipant | RemoteParticipant;

	interface Props {
		participants: Participant[];
		tiles: Participant[];
		version: number;
	}

	let { participants, tiles, version }: Props = $props();

	const screens = $derived.by(() => {
		void version;
		return participants.filter(isSharing).sort((a, b) => Number(a.isLocal) - Number(b.isLocal));
	});
	const featured = $derived(screens[0]);

	const rows = $derived(gridRows(tiles.length));
	const cols = $derived(gridCols(tiles.length, rows));
	const width = $derived(tileWidth(rows));
</script>

{#if featured}
	<main class="flex min-h-0 flex-1 flex-col gap-1">
		<div class="min-h-0 flex-1">
			<ParticipantTile participant={featured} isLocal={featured.isLocal} screen {version} />
		</div>
		<div class="flex shrink-0 gap-1 overflow-x-auto">
			{#each screens.slice(1) as p (p.identity)}
				<div class="w-48 shrink-0 aspect-video">
					<ParticipantTile participant={p} isLocal={p.isLocal} screen {version} />
				</div>
			{/each}
			{#each tiles as p (p.identity)}
				<div class="w-48 shrink-0">
					<ParticipantTile participant={p} isLocal={p.isLocal} {version} />
				</div>
			{/each}
		</div>
	</main>
{:else}
	<main
		class="grid min-h-0 flex-1 content-center gap-1 [container-type:size]"
		style:grid-template-columns="repeat({cols * 2}, minmax(0, 1fr))"
	>
		{#each tiles as p, i (p.identity)}
			<div class="flex min-w-0 justify-center" style:grid-column={gridColumn(i, tiles.length, rows, cols)}>
				<div style:width={width}>
					<ParticipantTile participant={p} isLocal={p.isLocal} {version} />
				</div>
			</div>
		{:else}
			<p class="col-span-full self-center text-center text-fc-p text-fc-muted">Connecting…</p>
		{/each}
	</main>
{/if}
