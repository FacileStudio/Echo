<script lang="ts">
	import type { LocalParticipant, RemoteParticipant } from 'livekit-client';
	import { gridCols, gridColumn, gridRows, tileWidth } from '$lib/grid';
	import { t } from '$lib/i18n/index.svelte';
	import { isSharing } from '$lib/screenShare';
	import { sameFocus, type Focus } from '$lib/focus';
	import ParticipantTile from './ParticipantTile.svelte';

	type Participant = LocalParticipant | RemoteParticipant;

	interface Props {
		participants: Participant[];
		tiles: Participant[];
		version: number;
		focus: Focus | null;
		onselect: (target: Focus) => void;
		onunpin: () => void;
	}

	let { participants, tiles, version, focus, onselect, onunpin }: Props = $props();

	const screens = $derived.by(() => {
		void version;
		return participants.filter(isSharing).sort((a, b) => Number(a.isLocal) - Number(b.isLocal));
	});
	const featured = $derived(focus ? participants.find((p) => p.identity === focus.identity) : undefined);
	const stripScreens = $derived(screens.filter((p) => !sameFocus(focus, { identity: p.identity, source: 'screen' })));
	const stripTiles = $derived(tiles.filter((p) => !sameFocus(focus, { identity: p.identity, source: 'camera' })));

	const rows = $derived(gridRows(tiles.length));
	const cols = $derived(gridCols(tiles.length, rows));
	const width = $derived(tileWidth(rows));

	const camera = (p: Participant): Focus => ({ identity: p.identity, source: 'camera' });
	const screen = (p: Participant): Focus => ({ identity: p.identity, source: 'screen' });
</script>

{#if featured && focus}
	<main class="flex min-h-0 min-w-0 flex-1 flex-col gap-1">
		<div class="min-h-0 flex-1">
			{#key focus.identity + focus.source}
				<ParticipantTile
					participant={featured}
					isLocal={featured.isLocal}
					screen={focus.source === 'screen'}
					focused
					{version}
					onselect={() => onselect(focus)}
					{onunpin}
				/>
			{/key}
		</div>
		<div class="flex shrink-0 gap-1 overflow-x-auto">
			{#each stripScreens as p (p.identity + ':screen')}
				<div class="w-36 shrink-0 aspect-video md:w-48">
					<ParticipantTile participant={p} isLocal={p.isLocal} screen {version} onselect={() => onselect(screen(p))} />
				</div>
			{/each}
			{#each stripTiles as p (p.identity)}
				<div class="w-36 shrink-0 md:w-48">
					<ParticipantTile participant={p} isLocal={p.isLocal} {version} onselect={() => onselect(camera(p))} />
				</div>
			{/each}
		</div>
	</main>
{:else}
	<main
		class="grid min-h-0 min-w-0 flex-1 content-center gap-1 [container-type:size]"
		style:grid-template-columns="repeat({cols * 2}, minmax(0, 1fr))"
	>
		{#each tiles as p, i (p.identity)}
			<div class="flex min-w-0 justify-center" style:grid-column={gridColumn(i, tiles.length, rows, cols)}>
				<div style:width={width}>
					<ParticipantTile participant={p} isLocal={p.isLocal} {version} onselect={() => onselect(camera(p))} />
				</div>
			</div>
		{:else}
			<p class="col-span-full self-center text-center text-fc-p text-fc-muted">{t('stage.connecting')}</p>
		{/each}
	</main>
{/if}
