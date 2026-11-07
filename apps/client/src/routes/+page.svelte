<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Alert, TextElevate, Button, Card, Field, Input, SignInWithFacile } from '@facile/muse';
	import { API_BASE, slugify } from '$lib/api';
	import { readKey } from '$lib/e2ee';
	import { t } from '$lib/i18n/index.svelte';
	import { loadSession, session } from '$lib/session.svelte';

	let room = $state('');
	let error = $state('');

	const loginError = $derived(page.url.searchParams.get('error') ?? '');

	onMount(() => {
		void loadSession();
	});

	function open(e: SubmitEvent) {
		e.preventDefault();
		const [path, fragment = ''] = room.split('#');
		const slug = slugify(path.split('/room/').pop() ?? '');
		const key = readKey(fragment);

		if (!slug) {
			error = t('home.roomRequired');
			return;
		}
		goto(key ? `/room/${slug}#key=${encodeURIComponent(key)}` : `/room/${slug}`);
	}
</script>

<svelte:head><title>Echo</title></svelte:head>

<main class="flex min-h-screen items-center justify-center px-4">
	<div class="flex w-full max-w-sm flex-col gap-2">
		<div class="flex items-center gap-2 text-center">
			<img src="/logo.svg" alt="" class="size-12" />
			<div>
				<h1 class="text-fc-2xl font-medium text-fc-fg"><TextElevate text="Echo" delay={0} /></h1>
			</div>
		</div>
		<p class="text-fc-sm text-fc-fg-muted">
			<TextElevate text={t('home.tagline')} delay={0.15} />
		</p>

		<Card class="mt-4">
			<form onsubmit={open} class="flex flex-col gap-1">
				{#if loginError}
					<Alert tone="danger" title={t('auth.failed')}>{loginError}</Alert>
				{/if}

				<Field label={t('home.roomName')} {error}>
					<Input bind:value={room} autocomplete="off" required />
				</Field>
				<Button type="submit" size="lg" class="w-full">{t('home.open')}</Button>
			</form>
		</Card>

		{#if session.loaded}
			<div class="flex justify-center">
				{#if session.me}
					<p class="text-fc-sm text-fc-fg-muted">{t('home.signedIn', { name: session.me.name || session.me.email })}</p>
				{:else}
					<SignInWithFacile href="{API_BASE}/api/auth/oidc" size="sm" />
				{/if}
			</div>
		{/if}
	</div>
</main>
