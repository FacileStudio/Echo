<script lang="ts">
	import { page } from '$app/state';
	import { Alert, AuthScreen } from '@facile/muse';
	import { API_BASE } from '$lib/api';
	import { t } from '$lib/i18n/index.svelte';

	const error = $derived(page.url.searchParams.get('error') ?? '');
</script>

<svelte:head><title>{t('title.login')}</title></svelte:head>

<AuthScreen appName="Echo" href="{API_BASE}/api/auth/oidc">
	{#snippet logo()}
		<img src="/logo.svg" alt="" class="size-12" />
	{/snippet}
	{#snippet tagline()}
		{t('home.tagline')}
	{/snippet}
	{#snippet notice()}
		{#if error}
			<Alert tone="danger" title={t('auth.failed')}>{error}</Alert>
		{/if}
	{/snippet}
</AuthScreen>
