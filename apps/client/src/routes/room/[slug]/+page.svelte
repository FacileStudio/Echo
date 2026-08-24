<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Alert, Button, Card, Field, Input, Page, PageHeader, Spinner } from '@facile/muse';
	import { fetchMe, requestToken, type Me, type TokenGrant } from '$lib/api';
	import LiveRoom from '$lib/components/LiveRoom.svelte';

	const slug = $derived(page.params.slug ?? '');

	let me = $state<Me | null>(null);
	let checked = $state(false);
	let grant = $state<TokenGrant | null>(null);
	let displayName = $state('');
	let error = $state('');

	onMount(async () => {
		me = await fetchMe();
		if (me) {
			displayName = me.name || me.email;
			await join();
		}
		checked = true;
	});

	async function joinGuest(e: SubmitEvent) {
		e.preventDefault();
		if (!me && displayName.trim()) await join();
	}

	async function join() {
		error = '';
		try {
			grant = await requestToken(slug, me ? undefined : displayName.trim());
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		}
	}
</script>

<svelte:head><title>Echo · {slug}</title></svelte:head>

{#if grant}
	<LiveRoom url={grant.url} token={grant.token} {displayName} {slug} />
{:else}
	<Page width="sm" gap="content">
		<PageHeader title="Join “{slug}”" description="Pick how you want to appear in the call." />

		{#if !checked}
			<Card><div class="flex justify-center py-4"><Spinner /></div></Card>
		{:else if error}
			<Alert tone="danger" title="Could not join">{error}</Alert>
			<Button href="/">Back</Button>
		{:else if me}
			<Card>
				<div class="flex items-center gap-4">
					<Spinner />
					<p class="text-fc-sm text-fc-fg-muted">Signed in as {displayName}. Joining…</p>
				</div>
			</Card>
		{:else}
			<Card>
				<form onsubmit={joinGuest} class="flex flex-col gap-4">
					<Field
						label="Display name"
						helper="Guests see only this name. Sign in to get moderator rights."
					>
						<Input bind:value={displayName} placeholder="Ada Lovelace" required minlength={2} />
					</Field>
					<Button type="submit" disabled={!slug || displayName.trim().length < 2}>Join as guest</Button>
					<Button variant="ghost" href="/" size="sm">Back to Echo</Button>
				</form>
			</Card>
		{/if}
	</Page>
{/if}
