<script lang="ts">
	import { Button, Icon, IconButton } from '@facile/muse';
	import { callIcons } from '$lib/icons';

	interface Props {
		micOn: boolean;
		camOn: boolean;
		screenOn: boolean;
		chatOpen: boolean;
		recording: boolean;
		canRecord: boolean;
		onToggleMic: () => void;
		onToggleCam: () => void;
		onToggleScreen: () => void;
		onToggleChat: () => void;
		onToggleRecording: () => void;
		onLeave: () => void;
	}

	let {
		micOn,
		camOn,
		screenOn,
		chatOpen,
		recording,
		canRecord,
		onToggleMic,
		onToggleCam,
		onToggleScreen,
		onToggleChat,
		onToggleRecording,
		onLeave
	}: Props = $props();

	const off = 'bg-fc-danger/10 text-fc-danger hover:bg-fc-danger/20';
</script>

<div class="flex items-center gap-1">
	<IconButton
		variant="ghost"
		class={micOn ? 'text-fc-ink' : off}
		aria-label={micOn ? 'Mute microphone' : 'Unmute microphone'}
		aria-pressed={!micOn}
		onclick={onToggleMic}
	>
		<Icon icon={micOn ? callIcons.mic : callIcons.micOff} />
	</IconButton>

	<IconButton
		variant="ghost"
		class={camOn ? 'text-fc-ink' : off}
		aria-label={camOn ? 'Turn camera off' : 'Turn camera on'}
		aria-pressed={!camOn}
		onclick={onToggleCam}
	>
		<Icon icon={camOn ? callIcons.cam : callIcons.camOff} />
	</IconButton>

	<IconButton
		variant="ghost"
		class={screenOn ? 'fc-selected' : ''}
		aria-label={screenOn ? 'Stop sharing screen' : 'Share screen'}
		aria-pressed={screenOn}
		onclick={onToggleScreen}
	>
		<Icon icon={callIcons.screen} />
	</IconButton>

	<IconButton
		variant="ghost"
		class="hidden md:inline-flex {chatOpen ? 'fc-selected' : ''}"
		aria-label={chatOpen ? 'Hide chat' : 'Show chat'}
		aria-pressed={chatOpen}
		onclick={onToggleChat}
	>
		<Icon icon={callIcons.chat} />
	</IconButton>

	{#if canRecord}
		<IconButton
			variant="ghost"
			class={recording ? off : ''}
			aria-label={recording ? 'Stop recording' : 'Start recording'}
			aria-pressed={recording}
			onclick={onToggleRecording}
		>
			<Icon icon={callIcons.record} />
		</IconButton>
	{/if}

	<Button variant="danger" size="lg" class="gap-2.5" onclick={onLeave}><Icon icon={callIcons.leave} />Leave</Button>
</div>
