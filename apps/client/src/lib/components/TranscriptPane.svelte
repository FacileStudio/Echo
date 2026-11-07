<script lang="ts">
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		transcript?: string;
	}

	let { transcript }: Props = $props();

	const text = $derived((transcript ?? '').trim());
</script>

{#if !text}
	<p class="text-fc-p text-fc-muted">{t('transcript.none')}</p>
{:else}
	<div
		class="max-h-96 overflow-y-auto rounded-fc bg-fc-surface p-4 text-fc-p whitespace-pre-wrap text-fc-ink"
	>
		{text}
	</div>
{/if}
