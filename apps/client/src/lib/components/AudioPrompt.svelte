<script lang="ts">
	import { Button } from '@facile/muse';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		blocked: boolean;
		onEnable: () => void;
	}

	let { blocked, onEnable }: Props = $props();
</script>

{#if blocked}
	<div class="px-4 pb-3">
		<Button size="sm" onclick={onEnable}>{t('audio.enable')}</Button>
	</div>
{/if}
