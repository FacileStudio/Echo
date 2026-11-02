<script lang="ts">
	export interface Caption {
		id: number;
		speaker: string;
		text: string;
	}

	let { captions = $bindable<Caption[]>([]) }: { captions: Caption[] } = $props();

	const visible = $derived(captions.slice(-3));
</script>

{#if visible.length > 0}
	<div class="pointer-events-none fixed bottom-28 left-1/2 z-10 w-full max-w-xl -translate-x-1/2 px-4">
		<div class="flex flex-col gap-1">
			{#each visible as c (c.id)}
				<p class="rounded-fc bg-fc-scrim px-3 py-2 text-fc-p text-fc-ink">
					<span class="text-fc-muted">{c.speaker}</span> {c.text}
				</p>
			{/each}
		</div>
	</div>
{/if}
