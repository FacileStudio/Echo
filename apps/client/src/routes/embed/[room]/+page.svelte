<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Alert } from '@facile/muse';
	import { t } from '$lib/i18n/index.svelte';
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

<svelte:head><title>{t('title.call')}</title></svelte:head>

{#if left}
	<main class="flex min-h-dvh items-center justify-center px-4">
		<p class="text-fc-sm text-fc-fg-muted">{t('embed.left')}</p>
	</main>
{:else if grant && !grant.key}
	<main class="flex min-h-dvh items-center justify-center px-4">
		<Alert tone="danger" title={t('embed.cannotJoin')}>{t('embed.noKey')}</Alert>
	</main>
{:else if grant}
	<LiveRoom
		url={grant.url}
		token={grant.token}
		e2eeKey={grant.key}
		displayName={grant.name}
		{slug}
		embedded
		onLeave={leave}
	/>
{:else if checked}
	<main class="flex min-h-dvh items-center justify-center px-4">
		<Alert tone="danger" title={t('embed.cannotJoin')}>{t('embed.noToken')}</Alert>
	</main>
{/if}
