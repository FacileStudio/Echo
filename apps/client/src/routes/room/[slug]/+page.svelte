<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Alert, TextElevate, Button, Card, Field, Input, Spinner } from '@facile/muse';
	import { requestToken, type Me, type TokenGrant } from '$lib/api';
	import { t } from '$lib/i18n/index.svelte';
	import { loadSession } from '$lib/session.svelte';
	import LiveRoom from '$lib/components/LiveRoom.svelte';
	import { roomKey } from '$lib/e2ee';
	import { PlanLimitError } from '$lib/planLimit';
	import PlanLimitNotice from '$lib/components/PlanLimitNotice.svelte';

	const slug = $derived(page.params.slug ?? '');

	let me = $state<Me | null>(null);
	let checked = $state(false);
	let grant = $state<TokenGrant | null>(null);
	let displayName = $state('');
	let key = $state('');
	let error = $state('');
	let planLimit = $state<PlanLimitError | null>(null);

	const historyHref = $derived(me && slug ? `/room/${slug}/history` : '');

	onMount(async () => {
		key = roomKey(slug);
		me = await loadSession();
		if (me) {
			displayName = me.name || me.email;
			await join();
		}
		checked = true;
	});

	async function joinGuest(e: SubmitEvent) {
		e.preventDefault();
		if (!me && displayName.trim())
			await join();
	}

	async function join() {
		error = '';
		planLimit = null;
		try {
			grant = await requestToken(slug, me ? undefined : displayName.trim());
		} catch (e) {
			if (e instanceof PlanLimitError) planLimit = e;
			else error = e instanceof Error ? e.message : String(e);
		}
	}
</script>

<svelte:head><title>{t('title.room', { slug })}</title></svelte:head>

{#if grant}
	<LiveRoom
		url={grant.url}
		token={grant.token}
		e2eeKey={key}
		{displayName}
		{slug}
		{historyHref}
		callDeadline={grant.call_deadline ?? ''}
		deadlineKey={grant.deadline_key ?? ''}
	/>
{:else}
	<main class="flex min-h-screen items-center justify-center px-4">
		<div class="flex w-full max-w-sm flex-col gap-1">
			<div class="flex items-center gap-2">
				<img src="/logo.svg" alt="" class="size-12" />
				<h1 class="min-w-0 break-words text-fc-2xl font-medium text-fc-fg"><TextElevate text={slug} delay={0} /></h1>
			</div>
			<p class="text-fc-sm text-fc-fg-muted">
				<TextElevate text={t('room.join')} delay={0.15} />
			</p>

			<Card class="mt-4">
				{#if !checked}
					<Spinner />
				{:else if planLimit}
					<div class="flex flex-col gap-1">
						<PlanLimitNotice error={planLimit} />
						<Button size="lg" variant="outline" class="w-full" onclick={() => void join()}>{t('room.tryAgain')}</Button>
					</div>
				{:else if error}
					<div class="flex flex-col gap-1">
						<Alert tone="danger" title={t('room.couldNotJoin')}>{error}</Alert>
						<Button href="/" size="lg" variant="outline" class="w-full">{t('room.back')}</Button>
						{#if historyHref}
							<Button href={historyHref} variant="ghost" class="w-full">{t('room.history')}</Button>
						{/if}
					</div>
				{:else if me}
					<div class="flex items-center gap-1">
						<Spinner />
						<p class="text-fc-sm text-fc-fg-muted">{t('room.joiningAs', { name: displayName })}</p>
					</div>
				{:else}
					<form onsubmit={joinGuest} class="flex flex-col gap-1">
						<Field label={t('room.yourName')}>
							<Input bind:value={displayName} autocomplete="name" required minlength={2} />
						</Field>
						<Button type="submit" size="lg" class="w-full" disabled={!slug || displayName.trim().length < 2}>{t('room.joinAsGuest')}</Button>
					</form>
					{/if}
				</Card>
				<Button href="/" variant="ghost" class="w-full">{t('room.backToEcho')}</Button>
		</div>
	</main>
{/if}
