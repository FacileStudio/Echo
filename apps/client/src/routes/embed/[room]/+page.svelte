<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Alert } from '@facile/muse';
	import LiveRoom from '$lib/components/LiveRoom.svelte';
	import { readEmbedGrant, type EmbedGrant } from '$lib/embed';

	const slug = $derived(page.params.room ?? '');

	let grant = $state<EmbedGrant | null>(null);
	let checked = $state(false);
	let left = $state(false);

	onMount(() => {
		grant = readEmbedGrant(window.location.hash);
		history.replaceState(history.state, '', window.location.pathname);
		checked = true;
	});

	function leave() {
		left = true;
		window.parent.postMessage({ type: 'echo.left', room: slug }, '*');
	}
</script>

<svelte:head><title>Call · Echo</title></svelte:head>

{#if left}
	<main class="flex min-h-dvh items-center justify-center px-4">
		<p class="text-fc-p text-fc-muted">You left the call.</p>
	</main>
{:else if grant}
	<LiveRoom url={grant.url} token={grant.token} displayName={grant.name} {slug} embedded onLeave={leave} />
{:else if checked}
	<main class="flex min-h-dvh items-center justify-center px-4">
		<Alert tone="danger" title="Cannot join">This call link has no join token. Open the call again from your app.</Alert>
	</main>
{/if}
