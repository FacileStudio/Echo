import { t } from './i18n/index.svelte';
import type { ChatMessage } from '$lib/components/ChatPanel.svelte';

export function parseChat(raw: string, fallbackName?: string): ChatMessage | null {
	try {
		const msg = JSON.parse(raw) as Partial<ChatMessage>;
		return {
			from: String(msg.from ?? ''),
			name: msg.name ?? fallbackName ?? t('chat.unknown'),
			text: String(msg.text ?? ''),
			at: Number(msg.at) || Date.now()
		};
	} catch {
		return null;
	}
}
