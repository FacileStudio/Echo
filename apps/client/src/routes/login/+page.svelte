<script lang="ts">
	import { page } from '$app/state';
	import { Alert, Button, Page, PageHeader } from '@facile/muse';

	const error = $derived(page.url.searchParams.get('error') ?? '');
</script>

<svelte:head><title>Sign in · Echo</title></svelte:head>

<Page width="sm" gap="content">
	<PageHeader title="Sign in" description="Echo uses the studio single sign-on." />

	{#if error}
		<Alert tone="danger" title="Login failed">{error}</Alert>
	{/if}

	<Button href="/">Back to Echo</Button>
</Page>
