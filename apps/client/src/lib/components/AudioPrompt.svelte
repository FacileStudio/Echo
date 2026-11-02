<script lang="ts">
	import { Button } from '@facile/muse';

	interface Props {
		blocked: boolean;
		onEnable: () => void;
	}

	let { blocked, onEnable }: Props = $props();
</script>

{#if blocked}
	<div class="px-4 pb-3">
		<Button size="sm" onclick={onEnable}>Enable sound</Button>
	</div>
{/if}
