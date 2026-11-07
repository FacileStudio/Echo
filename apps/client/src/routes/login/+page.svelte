<script lang="ts">
	import { page } from '$app/state';
	import { Alert, Button, Page, PageHeader } from '@facile/muse';
	import { API_BASE } from '$lib/api';
	import { t } from '$lib/i18n/index.svelte';

	const error = $derived(page.url.searchParams.get('error') ?? '');
</script>

<svelte:head><title>{t('title.login')}</title></svelte:head>

<Page width="sm" class="gap-4 px-4 py-10">
	<PageHeader title={t('login.signIn')} description={t('login.description')} />

	{#if error}
		<Alert tone="danger" title={t('auth.failed')}>{error}</Alert>
	{/if}

	<Button href="{API_BASE}/api/auth/oidc">{t('login.continue')}</Button>
</Page>
