<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button, Card, Field, Input, Alert, Page, PageHeader } from '@facile/muse';
	import { slugify } from '$lib/api';

	let roomName = $state('');
	let joinSlug = $state('');
	let joinError = $state('');

	const loginError = $derived(
		typeof page.url.searchParams.get('error') === 'string'
			? (page.url.searchParams.get('error') ?? '')
			: ''
	);

	function createRoom(e: SubmitEvent) {
		e.preventDefault();
		const slug = slugify(roomName);
		if (!slug) return;
		goto(`/room/${slug}`);
	}

	function joinRoom(e: SubmitEvent) {
		e.preventDefault();
		const slug = slugify(joinSlug);
		if (!slug) {
			joinError = 'Enter a room name or link';
			return;
		}
		goto(`/room/${slug}`);
	}
</script>

<svelte:head><title>Echo</title></svelte:head>

<Page width="sm" gap="content">
	{#if loginError}
		<Alert tone="danger" title="Login failed">{loginError}</Alert>
	{/if}

	<PageHeader
		title="Echo"
		description="Video rooms for the studio. Create a room or join one by name."
	/>

	<Card>
		<form onsubmit={createRoom} class="flex flex-col gap-4">
			<Field label="New room" helper="A readable name — it becomes the room address.">
				<Input bind:value={roomName} placeholder="Weekly sync" required />
			</Field>
			<Button type="submit">Create room</Button>
		</form>
	</Card>

	<Card>
		<form onsubmit={joinRoom} class="flex flex-col gap-4">
			<Field label="Join a room" error={joinError}>
				<Input bind:value={joinSlug} placeholder="weekly-sync" required />
			</Field>
			<Button type="submit" variant="outline">Join</Button>
		</form>
	</Card>
</Page>
