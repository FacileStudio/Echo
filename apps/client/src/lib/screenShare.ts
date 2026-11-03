import { ScreenSharePresets, Track, type LocalParticipant, type Participant } from 'livekit-client';

const preset = ScreenSharePresets.h1080fps30;

/** True when this browser can capture a screen, window or tab. */
export const canShareScreen = (): boolean =>
	typeof navigator !== 'undefined' && typeof navigator.mediaDevices?.getDisplayMedia === 'function';

/** True while the participant publishes a screen share video track. */
export const isSharing = (participant: Participant): boolean =>
	Boolean(participant.getTrackPublication(Track.Source.ScreenShare)?.videoTrack);

/**
 * Starts or stops sharing the screen in 1080p at 30 fps, with tab or system audio when offered.
 * Closing the picker or refusing the permission resolves to an empty string; any other capture
 * failure resolves to a message the call can show.
 */
export async function setScreenShare(participant: LocalParticipant, on: boolean): Promise<string> {
	try {
		await participant.setScreenShareEnabled(
			on,
			{
				audio: true,
				contentHint: 'detail',
				resolution: preset.resolution,
				selfBrowserSurface: 'exclude',
				surfaceSwitching: 'include',
				systemAudio: 'include'
			},
			{ screenShareEncoding: { ...preset.encoding, priority: 'high' }, degradationPreference: 'balanced' }
		);
		return '';
	} catch (e) {
		if (e instanceof DOMException && (e.name === 'NotAllowedError' || e.name === 'AbortError')) return '';
		return `Screen sharing failed: ${e instanceof Error ? e.message : String(e)}`;
	}
}
