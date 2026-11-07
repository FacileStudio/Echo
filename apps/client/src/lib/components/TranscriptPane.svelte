<script lang="ts">
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		transcript?: string;
	}

	let { transcript }: Props = $props();

	const text = $derived((transcript ?? '').trim());
</script>

{#if !text}
	<p class="text-fc-sm text-fc-fg-muted">{t('transcript.none')}</p>
{:else}
	<div
		class="max-h-96 overflow-y-auto rounded-fc-md bg-fc-surface p-4 text-fc-sm whitespace-pre-wrap text-fc-fg"
	>
		{text}
	</div>
{/if}
