<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Alert, TextElevate, Button, Card, Field, Input, Spinner } from '@facile/muse';
	import { fetchMe, requestToken, type Me, type TokenGrant } from '$lib/api';
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
		me = await fetchMe();
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

<svelte:head><title>{slug} · Echo</title></svelte:head>

{#if grant}
	<LiveRoom
		url={grant.url}
		token={grant.token}
		e2eeKey={key}
		{displayName}
		{slug}
		{historyHref}
		callDeadline={grant.call_deadline ?? ''}
	/>
{:else}
	<main class="flex min-h-screen items-center justify-center px-4">
		<div class="flex w-full max-w-sm flex-col gap-1">
			<div class="flex items-center gap-2">
				<img src="/logo.svg" alt="" class="size-12" />
				<h1 class="text-fc-title min-w-0 break-words text-fc-ink"><TextElevate text={slug} delay={0} /></h1>
			</div>
			<p class="text-fc-p text-fc-muted">
				<TextElevate text="Join the room." delay={0.15} />
			</p>

			<Card class="mt-4">
				{#if !checked}
					<Spinner />
				{:else if planLimit}
					<div class="flex flex-col gap-1">
						<PlanLimitNotice error={planLimit} />
						<Button size="lg" variant="outline" class="w-full" onclick={() => void join()}>Try again</Button>
					</div>
				{:else if error}
					<div class="flex flex-col gap-1">
						<Alert tone="danger" title="Could not join">{error}</Alert>
						<Button href="/" size="lg" variant="outline" class="w-full">Back</Button>
						{#if historyHref}
							<Button href={historyHref} variant="ghost" class="w-full">History</Button>
						{/if}
					</div>
				{:else if me}
					<div class="flex items-center gap-1">
						<Spinner />
						<p class="text-fc-p text-fc-muted">Joining as {displayName}…</p>
					</div>
				{:else}
					<form onsubmit={joinGuest} class="flex flex-col gap-1">
						<Field>
							<Input bind:value={displayName} placeholder="Your name" aria-label="Your name" class="text-fc-p" autocomplete="name" required minlength={2} />
						</Field>
						<Button type="submit" size="lg" class="w-full" disabled={!slug || displayName.trim().length < 2}>Join as guest</Button>
					</form>
					{/if}
				</Card>
				<Button href="/" variant="ghost" class="w-full">Back to Echo</Button>
		</div>
	</main>
{/if}
