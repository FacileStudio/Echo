<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Alert, TextElevate, Button, Card, Field, Input } from '@facile/muse';
	import { slugify } from '$lib/api';
	import { readKey } from '$lib/e2ee';

	let room = $state('');
	let error = $state('');

	const loginError = $derived(page.url.searchParams.get('error') ?? '');

	function open(e: SubmitEvent) {
		e.preventDefault();
		const [path, fragment = ''] = room.split('#');
		const slug = slugify(path.split('/room/').pop() ?? '');
		const key = readKey(fragment);

		if (!slug) {
			error = 'Enter a room name or paste a room link';
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
				<h1 class="text-fc-title text-fc-ink"><TextElevate text="Echo" delay={0} /></h1>
			</div>
		</div>
		<p class="text-fc-p text-fc-muted">
			<TextElevate text="Video rooms for the studio." delay={0.15} />
		</p>

		<Card class="mt-4">
			<form onsubmit={open} class="flex flex-col gap-1">
				{#if loginError}
					<Alert tone="danger" title="Login failed">{loginError}</Alert>
				{/if}

				<Field {error}>
					<Input bind:value={room} placeholder="Room name" aria-label="Room name" class="text-fc-p" autocomplete="off" required />
				</Field>
				<Button type="submit" size="lg" class="w-full">Open room</Button>
			</form>
		</Card>
	</div>
</main>
