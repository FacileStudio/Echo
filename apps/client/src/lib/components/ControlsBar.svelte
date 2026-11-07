<script lang="ts">
	import { Button, Icon, IconButton } from '@facile/muse';
	import { t } from '$lib/i18n/index.svelte';
	import { callIcons } from '$lib/icons';

	interface Props {
		micOn: boolean;
		camOn: boolean;
		screenOn: boolean;
		chatOpen: boolean;
		encrypted: boolean;
		canShare: boolean;
		onToggleMic: () => void;
		onToggleCam: () => void;
		onToggleScreen: () => void;
		onToggleChat: () => void;
		onLeave: () => void;
	}

	let {
		micOn,
		camOn,
		screenOn,
		chatOpen,
		encrypted,
		canShare,
		onToggleMic,
		onToggleCam,
		onToggleScreen,
		onToggleChat,
		onLeave
	}: Props = $props();

	const off = 'bg-fc-danger/10 text-fc-danger hover:bg-fc-danger/20';
	const on = 'bg-fc-accent text-fc-accent-fg hover:bg-fc-accent';
</script>

<div class="flex items-center gap-1">
	{#if encrypted}
		<span class="flex items-center gap-1 px-2 text-fc-xs text-fc-fg-muted" title={t('call.encryptedTip')}>
			<Icon icon={callIcons.lock} size={16} />
			<span class="sr-only md:not-sr-only">{t('call.encrypted')}</span>
		</span>
	{/if}

	<IconButton
		variant="ghost"
		class={micOn ? 'text-fc-fg' : off}
		aria-label={micOn ? t('call.mute') : t('call.unmute')}
		aria-pressed={!micOn}
		onclick={onToggleMic}
	>
		<Icon icon={micOn ? callIcons.mic : callIcons.micOff} />
	</IconButton>

	<IconButton
		variant="ghost"
		class={camOn ? 'text-fc-fg' : off}
		aria-label={camOn ? t('call.camOff') : t('call.camOn')}
		aria-pressed={!camOn}
		onclick={onToggleCam}
	>
		<Icon icon={camOn ? callIcons.cam : callIcons.camOff} />
	</IconButton>

	{#if canShare}
		<IconButton
			variant="ghost"
			class={screenOn ? on : ''}
			aria-label={screenOn ? t('call.stopShare') : t('call.share')}
			aria-pressed={screenOn}
			onclick={onToggleScreen}
		>
			<Icon icon={callIcons.screen} />
		</IconButton>
	{/if}

	<IconButton
		variant="ghost"
		class="hidden md:inline-flex {chatOpen ? on : ''}"
		aria-label={chatOpen ? t('call.hideChat') : t('call.showChat')}
		aria-pressed={chatOpen}
		onclick={onToggleChat}
	>
		<Icon icon={callIcons.chat} />
	</IconButton>

	<Button variant="danger" size="lg" class="gap-2.5" onclick={onLeave}><Icon icon={callIcons.leave} />{t('call.leave')}</Button>
</div>
