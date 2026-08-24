<script lang="ts">
	import { IconButton } from '@facile/muse';

	interface Props {
		micOn: boolean;
		camOn: boolean;
		screenOn: boolean;
		onToggleMic: () => void;
		onToggleCam: () => void;
		onToggleScreen: () => void;
		onLeave: () => void;
	}

	let { micOn, camOn, screenOn, onToggleMic, onToggleCam, onToggleScreen, onLeave }: Props =
		$props();
</script>

<div class="flex items-center justify-center gap-3">
	<IconButton
		variant={micOn ? 'default' : 'danger'}
		aria-label={micOn ? 'Mute microphone' : 'Unmute microphone'}
		onclick={onToggleMic}
	>
		<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
			<path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z" />
			<path d="M19 10v2a7 7 0 0 1-14 0v-2" />
			{#if !micOn}<line x1="2" y1="2" x2="22" y2="22" />{/if}
		</svg>
	</IconButton>

	<IconButton
		variant={camOn ? 'default' : 'danger'}
		aria-label={camOn ? 'Turn camera off' : 'Turn camera on'}
		onclick={onToggleCam}
	>
		<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
			<path d="m22 8-6 4 6 4V8Z" />
			<rect x="2" y="6" width="14" height="12" rx="2" />
			{#if !camOn}<line x1="2" y1="2" x2="22" y2="22" />{/if}
		</svg>
	</IconButton>

	<IconButton
		variant={screenOn ? 'default' : 'ghost'}
		aria-label={screenOn ? 'Stop sharing screen' : 'Share screen'}
		onclick={onToggleScreen}
	>
		<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
			<rect x="2" y="3" width="20" height="14" rx="2" />
			<line x1="8" y1="21" x2="16" y2="21" />
			<line x1="12" y1="17" x2="12" y2="21" />
		</svg>
	</IconButton>

	<button
		type="button"
		class="inline-flex h-11 items-center rounded-fc-pill bg-fc-danger px-5 text-fc-sm font-medium text-fc-danger-fg hover:opacity-90"
		onclick={onLeave}
	>
		Leave
	</button>
</div>
