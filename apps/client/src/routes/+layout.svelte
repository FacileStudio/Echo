<script lang="ts">
	import { provideLocale } from '@facile/muse';
	import favicon from '$lib/assets/favicon.svg';
	import { bindLocale, localeSource } from '$lib/i18n/index.svelte';
	import '../app.css';

	let { children } = $props();

	const i18n = provideLocale(localeSource);
	bindLocale(() => i18n.locale);
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
