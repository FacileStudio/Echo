import { RoomEvent, Track, type RemoteTrack, type Room } from 'livekit-client';

export function bindRemoteAudio(room: Room, onBlocked: (blocked: boolean) => void) {
	const mount = (track: RemoteTrack) => {
		if (track.kind === Track.Kind.Audio)
			document.body.appendChild(track.attach());
	};

	const unmount = (track: RemoteTrack) => {
		if (track.kind === Track.Kind.Audio)
			track.detach().forEach((el) => el.remove());
	};

	const status = () => onBlocked(!room.canPlaybackAudio);

	room
		.on(RoomEvent.TrackSubscribed, mount)
		.on(RoomEvent.TrackUnsubscribed, unmount)
		.on(RoomEvent.AudioPlaybackStatusChanged, status);

	return () => {
		room
			.off(RoomEvent.TrackSubscribed, mount)
			.off(RoomEvent.TrackUnsubscribed, unmount)
			.off(RoomEvent.AudioPlaybackStatusChanged, status);
	};
}
