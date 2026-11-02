import type { ChatMessage } from '$lib/components/ChatPanel.svelte';
import type { Caption } from '$lib/components/CaptionsOverlay.svelte';

export function parseCaption(raw: string, id: number): Caption | null {
	try {
		const msg = JSON.parse(raw) as { speaker?: string; text?: string };
		if (!msg.text) return null;
		return { id, speaker: String(msg.speaker ?? 'unknown'), text: String(msg.text) };
	} catch {
		return null;
	}
}

export function parseChat(raw: string, fallbackName?: string): ChatMessage | null {
	try {
		const msg = JSON.parse(raw) as Partial<ChatMessage>;
		return {
			from: String(msg.from ?? ''),
			name: msg.name ?? fallbackName ?? 'Unknown',
			text: String(msg.text ?? ''),
			at: Number(msg.at) || Date.now()
		};
	} catch {
		return null;
	}
}
