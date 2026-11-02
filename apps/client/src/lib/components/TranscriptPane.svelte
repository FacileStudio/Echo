<script lang="ts">
	interface Props {
		transcript?: string;
	}

	let { transcript }: Props = $props();

	const text = $derived((transcript ?? '').trim());
</script>

{#if !text}
	<p class="text-fc-p text-fc-muted">
		No transcript for this call. Transcription happens during the call, if it was turned on.
	</p>
{:else}
	<div
		class="max-h-96 overflow-y-auto rounded-fc bg-fc-surface p-4 text-fc-p whitespace-pre-wrap text-fc-ink"
	>
		{text}
	</div>
{/if}
